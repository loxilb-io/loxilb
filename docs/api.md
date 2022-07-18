# API server
Generic library for building a LoxiLB API server.

# Usage
현재 API 서버는 HTTP, HTTPS모두 지원하며, Loxilb 실행시 -a 혹은 --api 옵션을 추가해서 실행이 가능하다.
API에 사용되는 옵션은 다음과 같다. 보안을 위해 HTTPS 옵션 --tls-key, --tls-certificate를 모두 주어야만 실행이 가능하다.

Currently, the API server supports both HTTP and HTTPS, and can be run by adding -a or --api options when running Loxilb. The options used in the API are as follows. For security purposes, HTTPS options --tls-key, --tls-certificate must be given to run.

```
      --host=            the IP to listen on (default: localhost) [$HOST]
      --port=            the port to listen on for insecure connections, defaults to a random value [$PORT]
      --tls-host=        the IP to listen on for tls, when not specified it's the same as --host [$TLS_HOST]
      --tls-port=        the port to listen on for secure connections, defaults to a random value [$TLS_PORT]
      --tls-certificate= the certificate to use for secure connections [$TLS_CERTIFICATE]
      --tls-key=         the private key to use for secure connections [$TLS_PRIVATE_KEY]
```

실제 사용하는 예시는 다음과 같다.

Examples of practical use are as follows.

```
 ./loxilb --tls-key=api/certification/server.key --tls-certificate=api/certification/server.crt --host=0.0.0.0 --port=8081 --tls-port=8091 -a
```

# API list
현재 API는 Load balancer 에 대한 Create, Delete, Read API가 있다.
Currently, the API has Create, Delete, and Read APIs for Load balancer.

| Method | URL | Role | 
|------|---|---|
| GET|/netlox/v1/config/loadbalancer/all | Get the load balancer information |
| POST|/netlox/v1/config/loadbalancer| Add the load balancer information to LoxiLB |
| DELETE|/netlox/v1/config/loadbalancer/externalipaddress/{IPaddress}/port/{#Port}/protocol/{protocol} | Delete the load balacer infomation from LoxiLB|

더 자세한 정보( Param, Body 등)은 Swagger문서를 참조한다.

See Swagger documentation for more information (Param, Body, etc.).
