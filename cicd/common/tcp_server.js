var http = require('http');
var port = 8080
if (process.argv[3]) {
  port = 2020
}
http.createServer(function (req, res) {
  res.writeHead(200, {'Content-Type': 'text/html'});
  res.end(process.argv[2]);
}).listen(port);
