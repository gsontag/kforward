# kforward

[![CI](https://github.com/gsontag/kforward/actions/workflows/ci.yml/badge.svg)](https://github.com/gsontag/kforward/actions/workflows/ci.yml)

Keep your `kubectl port-forward`s running from the system tray.

kforward is a small Linux desktop application: each forward is a switch in
the tray menu, grouped as you like, and the icon shows how many are active.
When a pod goes away, the forward reconnects to the next ready one; when it
cannot, you get a notification.

![The window of kforward: forwards grouped by Databases, Monitoring and Other, three active and one failed with its error](docs/window.png)

## Features

- **Tray menu**: one switch per forward, optional groups, "Open in browser"
  for the forwards with a URL, "Stop all".
- **Icon badge**: the number of active forwards, green while connecting.
- **Window**: the state of every forward, and its last error.
- **Editor**: add, edit and delete forwards; namespaces, targets and ports
  are suggested from the cluster.
- **Reconnection**: follows the pods behind a service, a deployment or a
  statefulset when they are replaced; gives up after a few failures.
- **Notifications** when a forward stops for good, or when the configuration
  is wrong.
- **Live configuration**: the file is reloaded when edited by hand; only the
  forwards whose connection changed restart.
- **Log** of every change of state, in `~/.local/state/kforward/kforward.log`.
- **Start at login**, from the tray menu.
- English and French.

## Requirements

- Linux with a tray that supports StatusNotifierItem: KDE, XFCE, Cinnamon...
  On GNOME, the *AppIndicator and KStatusNotifierItem Support* extension
  (installed by default on Ubuntu).
- GTK 4 and GLib 2.88 or later, as in Ubuntu 26.04: the GTK bindings use
  recent GLib functions.
- A kubeconfig: kforward uses client-go, not the `kubectl` binary.

To build it:

- Go 1.27 or later
- a C compiler and `pkg-config` (the GTK bindings use cgo)
- the GTK 4 development files: `libgtk-4-dev` and `libgirepository1.0-dev`
  on Debian and Ubuntu, `gtk4-devel` on Fedora

## Install

```sh
git clone https://github.com/gsontag/kforward.git
cd kforward
make install
```

This installs, for the current user only:

- the program in `~/.local/bin/kforward`
- the launcher in `~/.local/share/applications`
- the icon in `~/.local/share/icons`

Use `make install PREFIX=/usr/local` (as root) for a system-wide install,
and `make uninstall` to remove it. The first build takes a few minutes: the
GTK bindings are large.

## Configuration

The forwards are stored in `~/.config/kforward/config.json`. The editor
writes it for you, but it is plain JSON:

```json
{
  "kubeconfig": "/home/me/.kube/config",
  "forwards": [
    {
      "uuid": "e1564734-b5df-43f4-826f-ee03d6d2931c",
      "name": "grafana",
      "group": "Monitoring",
      "context": "prod",
      "namespace": "monitoring",
      "target": "svc/grafana",
      "local-port": 3000,
      "remote-port": "http-web",
      "url": "http://localhost:3000",
      "auto-start": true
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `kubeconfig` | Optional. Without it, `$KUBECONFIG` or `~/.kube/config`, as for kubectl. |
| `uuid` | Identifies the forward: required and unique. The editor generates it. |
| `name` | Shown in the menu. |
| `group` | Optional. Forwards with the same group are shown together. |
| `context`, `namespace` | Optional. Default: the current context, and its namespace. |
| `target` | `pod/NAME` (or just `NAME`), `svc/NAME`, `deploy/NAME` or `sts/NAME`. |
| `local-port` | The port on your machine. |
| `remote-port` | A port number, or a port name of the pod or the service. |
| `address` | Optional. The address to listen on; default `127.0.0.1`. |
| `url` | Optional. Adds the forward to "Open in browser". |
| `auto-start` | Start the forward with the application. |

## Command line

```sh
kforward                 # start in the tray and open the window
kforward --background    # start in the tray only, as at login
kforward --check         # check each forward against the cluster, then exit
kforward --forward NAME  # run one forward in the terminal, until Ctrl-C
```

Only one instance runs: launching kforward again opens the window of the
running one.

## Development

```sh
make help            # lists the targets
make all             # quality checks, build and unit tests
make it              # integration tests, on a dedicated kind cluster
make i18n-extract    # after changing a text, then translate and make i18n-merge
```

The integration tests need [kind](https://kind.sigs.k8s.io/) and Docker.
They create a cluster named `kforward-it`, with its own kubeconfig in `out/`:
your current context is left untouched. `make it-clean` deletes it.

The GTK code (`internal/gui` and the `main` package) is kept thin and has no
unit tests: the menu, the window rows and the editor form are plain data
models, tested on their own.

## License

[MIT](LICENSE)
