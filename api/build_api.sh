sudo docker run --rm -it  --user $(id -u):$(id -g) -e GOPATH=$(go env GOPATH):/go -v $HOME:$HOME -w $(pwd) quay.io/goswagger/swagger:0.30.3 generate server
sed -i 's/s\.hasScheme(schemeHTTPS)/s\.hasScheme(schemeHTTPS) \&\& options\.Opts\.TLS/gi' restapi/server.go
sed -i'' -r -e '/import/a\\t\"github.com/loxilb-io/loxilb/options\"' restapi/server.go