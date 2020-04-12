# GoSimpleForwardingServer
 A proxy server that performs connection forwarding with the IP: PORT initially entered by the client

# Step
0. run server
1. client tcp connect to server
2. send ip:port after connection
3. server will forward it to ip:port

# Usage(Windows)
```
> set GOOS=Windows
> go Build
> GoSimpleForwardingServer.exe
```

# Usage(Linux)
```
> set GOOS=linux
> go Build
> chmod 755 GoSimpleForwardingServer
> ./GoSimpleForwardingServer
```