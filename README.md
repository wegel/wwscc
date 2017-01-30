```
                             Firewall                          Firewall
          Firewalled            or                                or
           network          HTTP Proxy                        HTTP Proxy
+-------------------------+     ++                                ++    +----------------------+
|    +---------------+    |     ||                                ||    |                      |
|    |               |    |     ||                                ||    |  +-----------------+ |
|    | Internal host |    |     ||                                ||    |  |                 | |
|    |               |    |     ||               +-------------------------+ 80  Web SSH     | |
|    |     22,80,... |    |     ||               |                ||    |  |                 | |
|    +---------^-----+    |     ||               |                ||    |  +-----------------+ | Firewalled
|              |          |     ||               |                ++    +----------------------+  network
|              |          |     ||               |
|    +---------+-------+  |     ||               |                ++    +----------------------+
|    |                 |  |     ||               |                ||    |                      |
|    |   Proxy      80 |  |     ||               |                ||    |  +-----------------+ |
|    |   (wwscat)      +-----+  ||    +----------v---------+      ||    |  |                 | |
|    |                 |  |  |  ||    |         80         |      ||    |  |      Tunnel     | |
|    +-----------------+  |  +--------> 80   Connector  80 <---------------+ 80  (wwscat)    | |
+-------------------------+     ++    |   (wwsconnector)   |      ||    |  |        +        | |
                                      |         80         |      ||    |  |        |        | |
+-------------------------+     ++    +----------^---------+      ||    |  |        v        | |
|   +-----------------+   |     ||               |                ||    |  |  ssh client,    | |
|   |                 |   |     ||               |                ||    |  |  browser, etc   | | Firewalled
|   |   Proxy      80 +--------------------------+                ||    |  +-----------------+ |  network
|   |   (wwscat)      |   |     ||                                ||    |  |-----------------| |
|   |                 |   |     ++                                ++    +----------------------+
|   +-----------------+   |  Firewall                          Firewall
+-------------------------+     or                                or
                            HTTP Proxy                        HTTP Proxy

```
Say we want to connect to a remote computer's SSH deamon that's not publicly available, but we have an existing communication channel to this computer that allows us to launch a command (or maybe this computer creates a channel on boot and automatically starts its "proxy").

Launch the *wwsconnector* somewhere publicly reachable:

`cd wwsconnector && go build && ./wwsconnector`

Obtain a Channel ID

``CHANNEL_ID=`curl http://public_wwsconnector_hostname/create` ``

On the "target" computer, the one which can reach the resource that we want to reach (the resource can be on that same computer), run *wwscat* in proxy mode:

`wwscat --proxy localhost:22 ws://public_wwsconnector_hostname/ws/proxy/$CHANNEL_ID`

On our local computer, we can do:

`ssh -C -D 1553 -o "VerifyHostKeyDNS=no" -o ProxyCommand="wwscat \"ws://public_wwsconnector_hostname/ws/tunnel/%h\"" root@$CHANNEL_ID`

And we'll be greeted by the standard SSH login prompt from the remote computer.

SSH is used as an example; you can proxy and connect to any TCP service.

You can also create a channel of type "SSH" (the default being "tunnel") where the *wwsconnector* will itself run an ssh client, bypassing the need to have an SSH client on our end. You would create the channel by specifying that you want an SSH tunnel:

``CHANNEL_ID=`curl http://public_wwsconnector_hostname/create?type=ssh` ``

You then would run the "proxy" exactly as above, and from our computer we could do:

``./wwscat "ws://public_wwsconnector_hostname/ws/tunnel/$CHANNEL_ID?username=ubuntu&rows=`tput lines`&cols=`tput cols`"``

You would then again be prompted with a password prompt, and eventually connected to the remote's shell.

This allows us to run a terminal using a web browser, since all the browser has to do is display the terminal. The SSH client runs on the wwsconnector. As an example, you can use wwswebterminal/terminal.html (and it's accompaning files). If you really want to or if you have no better place to host the web terminal, you can put the contents of *wwswebterminal* inside a *public* folder under *wwsconnector* and your connector will serve those files. 
