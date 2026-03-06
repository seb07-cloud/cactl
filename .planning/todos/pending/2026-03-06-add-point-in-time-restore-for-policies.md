---
created: 2026-03-06T14:10:34.133Z
title: Add point-in-time restore for policies
area: general
files:
  - cmd/rollback.go
  - internal/state/backend.go
---

## Problem

Currently rollback only supports reverting to the previous version of a policy. Users need the ability to restore policies to any arbitrary point in time — e.g., "restore all policies to how they looked last Tuesday at 3pm". This requires reading git blob history, parsing timestamps from commits/tags, presenting a timeline to the user, showing a complete diff of what would change, and then applying the restore.

## Solution

- Read git log/tag history to build a timeline of policy states with timestamps
- Parse commit dates and tag metadata to map each policy version to a point in time
- Add a `--at` or `--timestamp` flag to the rollback command (or a new `restore` subcommand)
- Show the user available restore points (list of dated snapshots)
- Generate a full diff between current state and the target point-in-time state
- Require confirmation before applying the restore
- Support restoring individual policies or all policies at once
- Leverage existing git blob reading from the state backend
