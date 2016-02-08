Example usage of simple Unix Datagram transport for Janus Gateway. 

## janus-info
Connects to gateway and issues the `info` Janus API request.

```
janus-info /path/to/unix/socket
```

## janus-list-sessions
Connects to gateway and issues the `list_sessions` Admin API request.

## Usage
```
janus-list-sessions /path/to/unix/socket [admin_secret]
```
