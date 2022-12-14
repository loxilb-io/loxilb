var http = require('http');
let count = 0;
http.createServer(function (req, res) {
  res.writeHead(200, {'Content-Type': 'text/html'});
  res.end('server2');
  count++;
  if (count == 1) {
    process.exit(0);
  }

}).listen(8080, "5ffe::1");
