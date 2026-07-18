# incus-lxc-provisioner design

A thin HTTP API that lets tenants manage their own LXC system containers on an Incus
cluster. This document records why it is shaped the way it is.

## Overview

It runs on a cluster member as a thin translator that turns a small set of tenant-scoped
requests into Incus REST calls and returns the result, nothing more. It does not
authenticate. A proxy in front does that. It holds no state and treats Incus's own state
as the source of truth. To a tenant it delivers an empty OS box and lets them drive its
lifecycle.

## Scope

An LXC system container boots as just an OS with no role. Unlike a Docker application
container, the image is not the role. Turning the server inside the box into a role is a
separate concern that lives outside this tool, so the responsibility narrows to handing
out empty boxes and keeping tenants apart.

Does:

- expose, per tenant, "create, delete, start, stop, list a container" over HTTP.
- translate to Incus REST. A single binary, configured by flags.

Does not:

- authenticate or authorize. The front proxy does that.
- hold state in the Incus backend. There is no database, and Incus is the source of
  truth. A memory backend exists for development only, see Backends.
- exec or console.
- wait for completion. It fires and returns, and state is read through the list.
- configure the inside of the box, or its reachability. Keys, cloud-init and connectivity
  are out of scope.
- metrics, retry, circuit-breaker, rate-limit, scheduling, HA decisions, backup.

## API and semantics

See the README for the endpoints and flags. This describes what the translation does.

The verb-to-Incus-REST mapping, verified against a live cluster:

| verb | Incus REST | kind | immediate response |
|---|---|---|---|
| create | `POST /1.0/instances?project=<p>` | async | 202 |
| delete | `DELETE /1.0/instances/{name}?project=<p>` | async | 202 / 4xx |
| start | `PUT /1.0/instances/{name}/state?project=<p>` `{"action":"start"}` | async | 202 / 4xx |
| stop | `PUT /1.0/instances/{name}/state?project=<p>` `{"action":"stop","force":true}` | async | 202 / 4xx |
| list | `GET /1.0/instances?project=<p>&recursion=2` | sync | 200 |

### fire-and-forget

Incus operations are asynchronous, and this tool does not wait for completion. It copies
only Incus's immediate accept or reject into HTTP. An accepted async request returns 202.
An error maps Incus's error_code to an HTTP status and is shaped as `{"error","message"}`.
The synchronous list returns 200. Completion is observed through the list, which uses
recursion=2 to bundle each instance's state and IP, so the IP is available in one call.

There is a known trade-off. If an accepted request fails asynchronously later, for example
on an image pull failure or an unusable profile, the instance is rolled back and never
appears in the list. "Still creating" and "failed" both read as absence and cannot be told
apart. This is accepted under a cattle/reconcile view: if it does not appear, create it
again.

### create takes only a name and an image

Create accepts a name and an image, nothing else. The box's shape, meaning its storage,
nic, resource limits and `security.*`, is fixed by the project's default profile. Neither a
profile choice nor a per-instance config override is accepted, so as not to break the
admin's envelope. This is covered under "The seam". Errors are shaped uniformly as
`{"error","message"}` by mapping Incus's error_code to a standard HTTP status, and stack
traces are not leaked.

## Tenant model and security

### group == project convention

The project is the caller's group name, with an optional prefix stripped by
`-tenant-group-prefix`. Every verb is scoped to the caller's project, and the tool always
passes `?project=<p>` when it calls Incus. There is no rules table that varies verbs per
group. Who is a tenant is decided by the operator creating the project and the group.

### trust boundary

The tool does not authenticate. It trusts the identity headers `X-Auth-Email` and
`X-Auth-Groups` set by the front proxy, which is the standard authenticating-proxy shape.
To be safe, the deployment must guarantee two things. First, the tool is reachable only
through the proxy. Second, the proxy overwrites any client-supplied header of the same
name, including variants such as `X_Auth_*`, with the authenticated value. If either
fails, any client can claim any group and so become any tenant.

### where isolation rests

The tool reaches Incus over the local unix socket. That socket is unrestricted and cannot
be scoped to a project, so Incus does not enforce a project boundary on the tool's
requests. Isolation rests on three layers: the proxy asserts only real groups, the tool
routes group to project, and the operator names the projects and groups. The
group-to-project resolution is therefore load-bearing, so it is kept simple and tested.
Not naming a tenant group after a shared project such as `default` is the operator's
responsibility.

### audit

Each mutating operation, meaning create, delete, start and stop, is logged as one JSON
line on stdout. What it records is the join from identity to project to verb to result.
The front proxy does not see the project and Incus does not see the SSO identity, so this
join is kept by neither, and the tool is the only place that has it. Reads, that is the
list, are not audited.

## Talking to Incus

The tool runs on a cluster member, a controller, and calls the Incus REST API over the
local unix socket using stdlib `net/http`. The unix socket speaks the same HTTP REST API
as the TCP endpoint, so the translation code is unchanged and no certificate or TLS is
needed. The connection to the local daemon is unauthenticated, and the daemon handles
cluster-wide operations internally.

It does not use the official `lxc/incus/client`. The exposed surface is only a few
instance paths and is stable, so stdlib is thinner than pulling in the heavy client
module, its behavior is followable with curl, and there is no dependency-tracking debt.

There is no Incus-side endpoint to bundle the controllers behind. The only remaining
network endpoint is the user-facing listener. For HA, run it on several controllers and
put the user-facing listeners behind a load balancer.

## Backends

The handlers do not call Incus directly. They call a `Backend` interface, and that
interface is where the implementations meet.

- **Incus** is the real backend. It calls the Incus REST API over the local unix socket
  and holds no state, since Incus is the source of truth.
- **Memory** keeps instances in memory. It exists so a client such as the web UI can be
  developed and demoed without an Incus cluster. It is selected with `-backend memory` and
  is not for production.

Both return the same view and the same errors, so the API contract is defined once, in the
handlers, and cannot drift between the two. This is also the seam the handler tests mock, so
the interface earns its place beyond the memory backend.

## The seam

What the tool needs in order to work is set up by the operator, and the tool only uses it.

- **admin, the cluster operator**: provides the project and the profile. A fresh project's
  default profile is empty, in the state `devices: {}`, with no root disk and no nic, so
  creates fail until the operator adds a root disk and a nic. That profile also fixes the
  box's size and its `security.*` posture.
- **API user, the tenant**: picks a name and an image and starts, stops and deletes their
  own boxes. The project is their group and the profile is the project's default, so they
  pick neither.
- **inside the box**: out of this tool's scope. After receiving an empty OS box, how the
  server inside it is set up is outside the tool. Reachability, whether you can get into
  the box, is likewise a property of the profile and image, and the tool takes no part.
