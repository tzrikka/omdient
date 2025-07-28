# Omdient

[![Go Reference](https://pkg.go.dev/badge/github.com/tzrikka/omdient.svg)](https://pkg.go.dev/github.com/tzrikka/omdient)
[![Go Report Card](https://goreportcard.com/badge/github.com/tzrikka/omdient)](https://goreportcard.com/report/github.com/tzrikka/omdient)

> [!NOTE]
> This repo is obsolete - see https://github.com/tzrikka/timpani/ (combination of Omdient and Ovid as a single binary)

Omdient is a robust and scalable listener for all kinds of asynchronous event notifications: via HTTP webhooks, [WebSocket](https://en.wikipedia.org/wiki/WebSocket) connections, and [Google Cloud Pub/Sub](https://cloud.google.com/pubsub/docs/overview).

Listeners may be passive and stateless receivers with a static configuration on the remote service's side, or semi-active subscribers that renew their subscription from time to time, or stateful clients maintaining a 2-way streaming connection with the remote service.

For example: event notifications from services such as Slack (simple HTTP webhook), Discord (WebSocket client), and Gmail (Pub/Sub subscriber).

Dependencies: secrets are managed by [Thrippy](https://github.com/tzrikka/thrippy), and horizontal scalability is managed by [etcd](https://etcd.io/).

Omdient is short for "omniaudient", which means "all-hearing".
