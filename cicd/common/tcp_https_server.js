// tcp_https_server.js

var certdir = "./"
if (process.argv[3]) {
  certdir = process.argv[3]
}
const https = require('https');
const fs = require('fs');

https.createServer({
    cert: fs.readFileSync(certdir + '/server.crt'),
    key: fs.readFileSync(certdir + '/server.key')
}, (req, res) => {
    res.writeHead(200);
    res.end(process.argv[2]);
}).listen(8080);
console.log("Server listening on https://localhost:8080/");
