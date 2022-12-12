#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <time.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <netinet/in.h>
#include <netinet/sctp.h>
#define MAX_BUFFER 1024
#define MY_PORT_NUM 38412 /* This can be changed to suit the need and should be same in server and client */

int
main (int argc, char* argv[])
{
  int listenSock, connSock, ret, in, flags, i;
  struct sockaddr_in servaddr;
  struct sctp_initmsg initmsg;
  struct sctp_event_subscribe events;
  struct sctp_sndrcvinfo sndrcvinfo;
  char buffer[MAX_BUFFER + 1];
  int count = 0;
  fd_set read_fd_set;

  listenSock = socket (AF_INET, SOCK_STREAM, IPPROTO_SCTP);
  if(listenSock == -1)
  {
      printf("Failed to create socket\n");
      perror("socket()");
      exit(1);
  }

  bzero ((void *) &servaddr, sizeof (servaddr));
  servaddr.sin_family = AF_INET;
  servaddr.sin_addr.s_addr = htonl (INADDR_ANY);
  servaddr.sin_port = htons (MY_PORT_NUM);

  ret = bind (listenSock, (struct sockaddr *) &servaddr, sizeof (servaddr));

  if(ret == -1 )
  {
      printf("Bind failed \n");
      perror("bind()");
      close(listenSock);
      exit(1);
  }

  /* Specify that a maximum of 5 streams will be available per socket */
  memset (&initmsg, 0, sizeof (initmsg));
  initmsg.sinit_num_ostreams = 5;
  initmsg.sinit_max_instreams = 5;
  initmsg.sinit_max_attempts = 4;
  ret = setsockopt (listenSock, IPPROTO_SCTP, SCTP_INITMSG,
      &initmsg, sizeof (initmsg));

  if(ret == -1 )
  {
      printf("setsockopt() failed \n");
      perror("setsockopt()");
      close(listenSock);
      exit(1);
  }

  ret = listen (listenSock, 5);
  if(ret == -1 )
  {
      printf("listen() failed \n");
      perror("listen()");
      close(listenSock);
      exit(1);
  }

  while (1)
  {
          char buffer[MAX_BUFFER + 1];
          int len;
          FD_ZERO(&read_fd_set);

          //Clear the buffer
          bzero (buffer, MAX_BUFFER + 1);

          //printf ("Awaiting a new connection\n");

          connSock = accept (listenSock, (struct sockaddr *) NULL, (int *) NULL);
          if (connSock == -1)
          {
                  printf("accept() failed\n");
                  perror("accept()");
                  close(connSock);
                  continue;
          }
          else
                  //printf ("Server: New client connected to %s\n", argv[1]);
          count++;
          FD_SET(connSock, &read_fd_set);
          int ret_val = select(FD_SETSIZE, &read_fd_set, NULL, NULL, NULL);
          if (ret_val >= 0) {
                  if (FD_ISSET(connSock, &read_fd_set)) {
                          in = sctp_recvmsg (connSock, buffer, sizeof (buffer),
                                          (struct sockaddr *) NULL, 0, &sndrcvinfo, &flags);

                          if( in == -1)
                          {
                                  printf("Server: Error in sctp_recvmsg\n");
                                  perror("sctp_recvmsg()");
                                  close(connSock);
                                  continue;
                          }
                          else
                          {
                                  //Add '\0' in case of text data
                                  buffer[in] = '\0';
                                  //printf (" Length of Data received: %d\n", in);
                                  //printf (" Data : %s\n", (char *) buffer);
                                  strncpy (buffer, argv[1], strlen(argv[1]));
                                  buffer[strlen(argv[1])] = '\0';
                                  int datalen = strlen(buffer);

                                  ret = sctp_sendmsg (connSock, (void *) buffer, (size_t) datalen,
                                                  NULL, 0, 0, 0, 0, 0, 0);
                                  if(ret == -1 )
                                  {
                                          printf("Error in sctp_sendmsg\n");
                                          perror("sctp_sendmsg()");
                                  }
                                  //else
                                    //      printf("Successfully sent %d bytes data to client\n", ret);
                          }
                          close (connSock);
                          if (count >= 5)
                              exit(0);
                  }
          }
  }

  return 0;
}
