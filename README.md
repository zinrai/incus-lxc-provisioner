# incus-lxc-provisioner

A thin HTTP API that provisions LXC system containers to tenants on an Incus cluster. It
runs on a cluster member and translates a small, tenant-scoped set of requests into Incus
REST calls, nothing more. It does not authenticate. A proxy in front does. It holds no
state, since Incus is the source of truth. Design rationale and the Incus REST mapping are
in [DESIGN.md](DESIGN.md).

```
GET    /containers              list the tenant's containers (and their state)
POST   /containers              create
DELETE /containers/{name}       delete (stop it first)
POST   /containers/{name}/start start
POST   /containers/{name}/stop  stop
GET    /healthz                 liveness (no auth)
```

Create takes only a name and an image. The box's shape, such as its storage, nic, limits
and security posture, is the incus admin's, fixed by the project's default profile. The
provisioner hands over an empty OS box and manages its lifecycle. Setting up the server
inside it is out of scope. It fires each request and returns without waiting, so state is
read back through the list.

## Run

It runs on an Incus cluster member and talks to the local daemon over the unix socket, so
it needs no certificates. Run it as root or in the `incus` group:

```
./incus-lxc-provisioner -incus-socket /var/lib/incus/unix.socket
```

`-incus-socket` defaults to `/var/lib/incus/unix.socket`, so `./incus-lxc-provisioner` on
its own usually works. It is a single static binary configured by command-line flags, and
you supervise it however you run long-lived processes. It listens on localhost behind an
authentication proxy such as oauth2-proxy that sets `X-Auth-Email` and `X-Auth-Groups`. The
provisioner trusts those and maps the group to an Incus project of the same name. Strip a
prefix with `-tenant-group-prefix`.

For development, or to demo a client such as a UI without an Incus cluster, run it with
`-backend memory`. The HTTP contract is identical, but instances are kept in memory instead
of created on Incus. This mode is not for production.

## What the incus admin sets up

`project` and `profile` are the incus admin's. The provisioner only uses them and never
creates or changes them. Per tenant the admin:

- creates the project with `incus project create team-a`.
- gives that project's default profile a root disk and a nic. A fresh project's default
  profile is empty, `devices: {}`, and containers cannot be created until this is done.
  This profile also fixes the box's size and `security.*` posture.

A user of the API never picks a project, which is their group's, or a profile, whose shape
is the project's default profile. They only manage their own boxes.

## Security

The provisioner trusts the `X-Auth-*` headers completely and enforces no project boundary
of its own beyond routing the caller's group to a project. Two conditions must hold, or any
client can act as any tenant:

- The provisioner must be reachable only through the proxy, never directly.
- The proxy must overwrite any client-supplied `X-Auth-*` header, including underscore and
  case variants such as `X_Auth_Groups`, with the authenticated value.

Because the local socket gives Incus no per-request project enforcement, isolation rests on
the proxy asserting only real groups, the provisioner's group-to-project routing, and the
operator's project and group names. Do not name a tenant group after a shared project like
`default`.

## License

This project is licensed under the [MIT License](LICENSE).
