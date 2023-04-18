#include <sys/socket.h>
#include <sys/types.h>
#include <netinet/in.h>
#include <netinet/sctp.h>
#include <arpa/inet.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>

#define RECVBUFSIZE     4096
#define PPID            1234

int main(int argc, char* argv[])
{

        struct sockaddr_in servaddr = {0};
        struct sockaddr_in laddr = {0};
        int    sockfd, in, flags;
        char   *saddr;
        int    sport, lport, error = 0;
        struct sctp_status status = {0};
        struct sctp_sndrcvinfo sndrcvinfo = {0};
        struct sctp_event_subscribe events = {0};
        struct sctp_initmsg initmsg = {0};
        char   msg[1024]  = {0};
        char   buff[1024] = {0};
        socklen_t opt_len;
        socklen_t slen = (socklen_t) sizeof(struct sockaddr_in);


        sockfd = socket(AF_INET, SOCK_STREAM, IPPROTO_SCTP);
        lport = atoi(argv[2]);

        laddr.sin_family = AF_INET;
        laddr.sin_addr.s_addr = inet_addr(argv[1]);
        laddr.sin_port = lport?htons(lport):0;

        //bind to local address
        error = bind(sockfd, (struct sockaddr *)&laddr, sizeof(struct sockaddr_in));
        if (error != 0) {
            printf("\n\n\t\t***r: error binding addr:"
            " %s. ***\n", strerror(errno));
            exit(1);
       }

        //set the association options
        initmsg.sinit_num_ostreams = 1;
        setsockopt( sockfd, IPPROTO_SCTP, SCTP_INITMSG, &initmsg,sizeof(initmsg));

        saddr = argv[3];
        sport = atoi(argv[4]);
        bzero( (void *)&servaddr, sizeof(servaddr) );
        servaddr.sin_family = AF_INET;
        servaddr.sin_port = htons(sport);
        servaddr.sin_addr.s_addr = inet_addr( saddr );


        connect( sockfd, (struct sockaddr *)&servaddr, sizeof(servaddr));

        opt_len = (socklen_t) sizeof(struct sctp_status);
        getsockopt(sockfd, IPPROTO_SCTP, SCTP_STATUS, &status, &opt_len);

        while(1)
        {
                strncpy (msg, "hello", strlen("hello"));
                //printf("Sending msg to server: %s", msg);
                sctp_sendmsg(sockfd, (const void *)msg, strlen(msg), NULL, 0,htonl(PPID), 0, 0 , 0, 0);

                in = sctp_recvmsg(sockfd, (void*)buff, RECVBUFSIZE, 
                                  (struct sockaddr *)&servaddr, 
                                  &slen, &sndrcvinfo, &flags);
                if (in > 0 && in < RECVBUFSIZE - 1)
                {
                        buff[in] = 0;
                        printf("%s",buff);
                        break;
                }
        } 

        close(sockfd);
        return 0;
}
