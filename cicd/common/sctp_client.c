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
        struct sockaddr_in laddr[10] = {0};
        int    sockfd, in, flags, count = 1;
        char   *saddr, *laddrs, *addr;
        int    sport, lport, error = 0, secs = 0, i = 0;
        struct sctp_status status = {0};
        struct sctp_sndrcvinfo sndrcvinfo = {0};
        struct sctp_event_subscribe events = {0};
        struct sctp_initmsg initmsg = {0};
        char   msg[1024]  = {0};
        char   buff[1024] = {0};
        socklen_t opt_len;
        socklen_t slen = (socklen_t) sizeof(struct sockaddr_in);

        if (argc < 5) {
            printf("Usage: %s localaddr1<,localaddr2..> localport serverIP serverport <delay> <count>\n", argv[0]);
            exit(1);
        }
        sockfd = socket(AF_INET, SOCK_STREAM, IPPROTO_SCTP);
        lport = atoi(argv[2]);
        laddrs = argv[1];
        memset(laddr, 0, sizeof(laddr));
        if (strstr(laddrs, ",")) {
            i = 0;
            addr = strtok(laddrs, ",\n");
            laddr[0].sin_family = AF_INET;
            laddr[0].sin_port = lport?htons(lport):0;
            laddr[0].sin_addr.s_addr = inet_addr(addr);
            //printf("%s\n", addr);
            addr = strtok(NULL, ",\n");
            while(addr != NULL) {
                i++;
                //printf("%s\n", addr);
                laddr[i].sin_family = AF_INET;
                laddr[i].sin_port = lport?htons(lport):0;
                laddr[i].sin_addr.s_addr = inet_addr(addr);
                addr = strtok(NULL, ",\n");
            }
       } else {
        laddr[0].sin_family = AF_INET;
        laddr[0].sin_addr.s_addr = inet_addr(argv[1]);
        laddr[0].sin_port = lport?htons(lport):0;
       }

        //bind to local address
        error = bind(sockfd, (struct sockaddr *)&laddr, sizeof(struct sockaddr_in));
        if (error != 0) {
            printf("\n\n\t\t***r: error binding addr:"
            " %s. ***\n", strerror(errno));
            exit(1);
       }

       int j = 1;
       while(j <= i) {
               error = sctp_bindx(sockfd,(struct sockaddr*) &laddr[j], j, SCTP_BINDX_ADD_ADDR);
               if (error != 0) {
                       printf("\n\n\t\t***r: error adding addrs:"
                                       " %s. ***\n", strerror(errno));
                       exit(1);
               } else {
                   //printf("Bind OK\n");
               }
               j++;
       }

        //set the association options
        initmsg.sinit_num_ostreams = 1;
        setsockopt( sockfd, IPPROTO_SCTP, SCTP_INITMSG, &initmsg,sizeof(initmsg));

        saddr = argv[3];
        sport = atoi(argv[4]);
        if (argc >=6 ) { /* Delay before exit */
           secs = atoi(argv[5]);
        }
        if (argc == 7) { /* count for sending 1pps*/
           count = atoi(argv[6]);
        }
        bzero( (void *)&servaddr, sizeof(servaddr) );
        servaddr.sin_family = AF_INET;
        servaddr.sin_port = htons(sport);
        servaddr.sin_addr.s_addr = inet_addr( saddr );


        connect( sockfd, (struct sockaddr *)&servaddr, sizeof(servaddr));

        opt_len = (socklen_t) sizeof(struct sctp_status);
        getsockopt(sockfd, IPPROTO_SCTP, SCTP_STATUS, &status, &opt_len);

        fd_set fds; // will be checked for being ready to read
        FD_ZERO(&fds);
        FD_SET(sockfd, &fds);
        struct timeval tv = { 0 };
        tv.tv_sec = 1;
        tv.tv_usec = 0;

        while(count)
        {
                strncpy (msg, "hello", strlen("hello"));
                sctp_sendmsg(sockfd, (const void *)msg, strlen(msg), NULL, 0,htonl(PPID), 0, 0 , 0, 0);
                //printf("Sending msg to server: %s", msg);
                //
                int ret = select( sockfd + 1, &fds, NULL, NULL, &tv );
                if (ret <= 0) {
                    printf("Timeout\n");
                    count--;
                    sleep(1);
                } else if (FD_ISSET( sockfd, &fds )) {

                in = sctp_recvmsg(sockfd, (void*)buff, RECVBUFSIZE, 
                                  (struct sockaddr *)&servaddr, 
                                  &slen, &sndrcvinfo, &flags);
                if (in > 0 && in < RECVBUFSIZE - 1)
                {
                        buff[in] = 0;
                        printf("%s",buff);
                        fflush(stdout);
                        count--;
                        if(!count)
                            break;
                        else {
                            printf("\n");
                            fflush(stdout);
                            sleep(1);
                        }
                }
                } else {
                    break;
                }
        } 
        if(secs) sleep(secs);

        close(sockfd);
        return 0;
}
