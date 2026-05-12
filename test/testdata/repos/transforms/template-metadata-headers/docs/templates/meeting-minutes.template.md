---
title: Meeting minutes template
params:
  docbuilder:
    template:
      type: general-meeting-minutes
      name: Meeting Minutes
      output_path: minutes/{{ .Date }}-{{ .Slug }}.md
      description: Create a meeting minutes document
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'
      defaults: '{"categories":["minutes"]}'
---

# Meeting minutes template

```markdown
---
title: "{{ .Title }} - {{ .Date }}"
slug: "{{ .Slug }}"
---

# {{ .Title }}
```
