---
title: "REST API"
linkTitle: "REST API"
weight: 1
description: >
  RESTful API reference documentation
type: docs
---

# REST API

Complete REST API documentation.

## Authentication

All API requests require authentication.

### API Keys

Use your API key in the Authorization header:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  https://api.example.com/v1/users
```

## Rate Limiting

API calls are rate limited to 1000 requests per hour.
