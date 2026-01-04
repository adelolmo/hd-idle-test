# hd-idle test suite

Tool to help testing hd-idle for real usecase scenarios.

_Requirements:_
- Go: https://go.dev/doc/install
- Preferred Systemd

## Daemon

To build it, use the command:

```
$ make
# make install
```

## TUI

To build it, use the command:

```
$ make
# make install
```

## Setup a usecase

Make sure you stop hd-idle service before running the usecase. e.g. `systemctl stop hd-idle`

Start hd-idle in the terminal with the argument `-l /var/log/hd-idle.log` and redirect the output to a file `/tmp/hd-idle.out`.

e.g.
```
# nohup ./hd-idle -l /var/log/hd-idle.log > /tmp/hd-idle.out
```

Start the daemon if not ard already running. e.g. `systemctl start hdtd`

Start the TUI: `hdt`

Now you can start the recording pressing `r` and run your usecase (Notice the `R` in the bottom left corner). The daemon will record the events. You can press `F5` to load the recorded events right away.

## Navigation

![TUI Screenshot](screenshot.png)

On the left panel you can see the available recorded sessions. Navigate up and down using the arrows, and select one pressing `Enter`.

On the right panel you can see the events of the selected session. Navigate through time using the right arrow to advance and the left arrow to go back.
`Shift + right arrow` to go forward 10 pages and `Ctrl + right arrow` to go back 100 pages (also valid for left arrow). 