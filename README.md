Example usage of simple Unix Datagram transport for Janus Gateway. 

## info
Connects to gateway and issues the `info` Janus API request.

```
janus-info /path/to/unix/socket
```

## echotest
Starts an echotest server. See configuration options in echotest/echotest.cfg

```
echotest /path/to/echotest.cfg
```

## list-sessions
Connects to gateway and issues the `list_sessions` Admin API request.

```
janus-list-sessions /path/to/unix/socket [admin_secret]
```
