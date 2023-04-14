#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/sctp.h>
#include <arpa/inet.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>

#define RECVBUFSIZE             4096
#define PPID                    1234

int main(int argc, char* argv[]) {

       struct sockaddr_in laddr = {0};
       struct sockaddr_in caddr = {0};
       int    sockfd, n, flags;
       struct sctp_sndrcvinfo sinfo = {0};
       struct sctp_event_subscribe event = {0};
       char recvbuff[RECVBUFSIZE + 1] = {0};
       socklen_t clen;

       char *saddr, *msg;
       int lport, mlen;

       saddr = argv[1];
       lport = atoi(argv[2]);
       msg = argv[3];

       mlen = strlen(msg);

       sockfd = socket(AF_INET, SOCK_SEQPACKET, IPPROTO_SCTP);

       setsockopt(sockfd, IPPROTO_SCTP, SCTP_EVENTS, &event,sizeof(struct sctp_event_subscribe));

       laddr.sin_family = AF_INET;
       laddr.sin_port = htons(lport);
       laddr.sin_addr.s_addr = inet_addr(saddr);

       bind(sockfd, (struct sockaddr *)&laddr, sizeof(struct sockaddr_in));

       listen(sockfd, 1);

       while(1)
       {
               flags = 0;
               memset((void *)&caddr, 0, sizeof(struct sockaddr_in));
               clen = (socklen_t)sizeof(struct sockaddr_in);
               memset((void *)&sinfo, 0, sizeof(struct sctp_sndrcvinfo));

               n = sctp_recvmsg(sockfd, (void*)recvbuff, RECVBUFSIZE,(struct sockaddr *)&caddr, &clen, &sinfo, &flags);
               if (-1 == n)
               {
                       printf("Error with sctp_recvmsg: %d\n", errno);
                       perror("Description: ");
                       printf("Waiting..\n");
                       sleep(1);
                       continue;
               }

               if (flags & MSG_NOTIFICATION)
               {
                       printf("Notification received from %s:%u\n", inet_ntoa(caddr.sin_addr), ntohs(caddr.sin_port));
               }
               else
               {
                       printf("Received from %s:%u on stream %d, PPID %d.: %s\n",
                               inet_ntoa(caddr.sin_addr),
                               ntohs(caddr.sin_port),
                               sinfo.sinfo_stream,
                               ntohl(sinfo.sinfo_ppid),
                               recvbuff);
               }

               printf("Sending msg to client: %s\n", msg);
               sctp_sendmsg(sockfd, (const void *)msg, strlen(msg), (struct sockaddr *)&caddr, clen, htonl(PPID), 0, 0 , 0, 0);

       }//while

       close(sockfd);
       return (0);
}
 
