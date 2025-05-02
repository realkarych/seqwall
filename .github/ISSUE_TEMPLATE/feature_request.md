name: Feature Request
description: Propose a new idea or improvement
title: "[Feature] "
labels: [enhancement]
assignees: []

body:
  - type: textarea
    id: motivation
    attributes:
      label: Why do you need this?
      description: What problem would this solve or improve?
    validations:
      required: true

  - type: textarea
    id: solution
    attributes:
      label: What’s your proposed solution?
      description: Describe the desired behavior, CLI flags, or configuration
    validations:
      required: true

  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives
      description: Have you considered other solutions or workarounds?
    validations:
      required: false
