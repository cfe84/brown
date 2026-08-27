"use strict";

const http = require("http");

const port = Number(process.env.PORT || 8080);
const chunk = Buffer.alloc(256 * 1024, 0x42);

const server = http.createServer((request, response) => {
  if (request.method === "GET" && request.url === "/healthz") {
    response.writeHead(200, { "Content-Type": "text/plain" });
    response.end("ok\n");
    return;
  }

  if (request.method === "GET" && request.url === "/download") {
    response.writeHead(200, { "Content-Type": "application/octet-stream" });
    const write = () => {
      while (!response.destroyed && response.write(chunk)) {}
      if (!response.destroyed) response.once("drain", write);
    };
    write();
    request.on("close", () => response.destroy());
    return;
  }

  if (request.method === "POST" && request.url === "/upload") {
    request.on("error", () => response.destroy());
    request.on("end", () => {
      if (!response.destroyed) {
        response.writeHead(200, { "Content-Type": "text/plain" });
        response.end("ok\n");
      }
    });
    request.resume();
    return;
  }

  response.writeHead(404, { "Content-Type": "text/plain" });
  response.end("not found\n");
});

server.listen(port, "0.0.0.0", () => {
  console.log(`Brown bandwidth server listening on port ${port}`);
});
