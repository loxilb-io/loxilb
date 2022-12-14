#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <netinet/in.h>
#include <netinet/sctp.h>
#include <arpa/inet.h>
#define MAX_BUFFER 1024
#define MY_PORT_NUM 8080 /* This can be changed to suit the need and should be same in server and client */

int
main (int argc, char* argv[])
{
  int connSock, in, i, ret, flags;
  struct sockaddr_in servaddr;
  struct sctp_status status;
  struct sctp_sndrcvinfo sndrcvinfo;
  char buffer[MAX_BUFFER + 1];
  int datalen = 0;
  fd_set read_fd_set;

  /*Get the input from user*/
  //printf("Enter data to send: ");
  //fgets(buffer, MAX_BUFFER, stdin);
  /* Clear the newline or carriage return from the end*/
  //buffer[strcspn(buffer, "\r\n")] = 0;
  /* Sample input */
  strncpy (buffer, "Hello Server", 12);
  buffer[12] = '\0';
  datalen = strlen(buffer);

  connSock = socket (AF_INET, SOCK_STREAM, IPPROTO_SCTP);

  if (connSock == -1)
  {
      printf("Socket creation failed\n");
      perror("socket()");
      exit(1);
  }

  bzero ((void *) &servaddr, sizeof (servaddr));
  servaddr.sin_family = AF_INET;
  servaddr.sin_port = htons (atoi(argv[2]));
  servaddr.sin_addr.s_addr = inet_addr (argv[1]);

  ret = connect (connSock, (struct sockaddr *) &servaddr, sizeof (servaddr));

  if (ret == -1)
  {
      printf("Connection failed\n");
      perror("connect()");
      close(connSock);
      exit(1);
  }
  FD_ZERO(&read_fd_set);
  ret = sctp_sendmsg (connSock, (void *) buffer, (size_t) datalen,
        NULL, 0, 0, 0, 0, 0, 0);
  if(ret == -1 )
  {
    printf("Error in sctp_sendmsg\n");
    perror("sctp_sendmsg()");
  }
  else {
   //       printf("Successfully sent %d bytes data to server\n", ret);
          FD_SET(connSock, &read_fd_set);
          int ret_val = select(FD_SETSIZE, &read_fd_set, NULL, NULL, NULL);
          if (ret_val >= 0) {
              if (FD_ISSET(connSock, &read_fd_set)) {

                  in = sctp_recvmsg (connSock, buffer, sizeof (buffer),
                          (struct sockaddr *) NULL, 0, &sndrcvinfo, &flags);

                  if( in == -1)
                  {
                    printf("Error in sctp_recvmsg\n");
                    perror("sctp_recvmsg()");
                    close(connSock);
                    return -1;
                  }
                  else {
                    //Add '\0' in case of text data
                    buffer[in] = '\0';

                    // printf (" Length of Data received: %d\n", in);
                    printf ("%s", (char *) buffer);
                  }
            }
       }
  }

  close (connSock);

  return 0;
}
