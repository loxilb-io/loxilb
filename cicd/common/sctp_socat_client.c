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

#define RECVBUFSIZE     1024
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
        char   msg[RECVBUFSIZE]  = {0};
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

        saddr = argv[3];
        sport = atoi(argv[4]);
        bzero( (void *)&servaddr, sizeof(servaddr) );
        servaddr.sin_family = AF_INET;
        servaddr.sin_port = htons(sport);
        servaddr.sin_addr.s_addr = inet_addr( saddr );

        connect( sockfd, (struct sockaddr *)&servaddr, sizeof(servaddr));

        while(1)
        {
                in = recv(sockfd, (void*)msg, RECVBUFSIZE, 0);
                if (in > 0 && in < RECVBUFSIZE - 1)
                {
                        msg[in] = 0;
                        printf("%s",msg);
                        break;
                }
        } 

        close(sockfd);
        return 0;
}
