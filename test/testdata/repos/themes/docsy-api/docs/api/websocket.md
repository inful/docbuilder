---
title: "WebSocket API"
linkTitle: "WebSocket"
weight: 2
description: >
  Real-time WebSocket API documentation
type: docs
---

# WebSocket API

Real-time communication via WebSocket.

## Connection

Connect to the WebSocket endpoint:

```javascript
const ws = new WebSocket('wss://api.example.com/ws');
```

## Events

Subscribe to events:

- `user.created` - New user created
- `user.updated` - User updated
- `user.deleted` - User deleted
