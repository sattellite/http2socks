# http2socks

A simple HTTP proxy server that connects to a SOCKS proxy
server and passes traffic through it.

To use it, you need to specify the address that will
listen to http proxy server. Also it is necessary to
specify the address of socks server and username and
password to access it.

Data for start can be passed by flags or environment
variables.  

| Name                  | Flag                    | Environment            |
|-----------------------|-------------------------|------------------------|
| HTTP proxy address    | `-http_address`         | `HTTP_ADDRESS`         |
| SOCKS5 proxy server   | `-socks_proxy`          | `SOCKS_PROXY`          |
| SOCKS5 proxy user     | `-socks_proxy_user`     | `SOCKS_PROXY_USER`     |
| SOCKS5 proxy password | `-socks_proxy_password` | `SOCKS_PROXY_PASSWORD` |
