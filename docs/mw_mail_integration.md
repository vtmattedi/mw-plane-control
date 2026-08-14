# MW Mail project deletion integration

Status: implemented downstream and mapped by MW BFF.

MW Mail now exposes the administrative project-deletion contract requested by
MW Control Plane.

## Required service endpoint

```http
DELETE /admin/api/projects/{id}
Authorization: Bearer <delegated-token>
```

Required delegated scope: `mail.projects.delete`.

Expected behavior:

- Return `204 No Content` after deleting an existing project.
- Return `404` when the project does not exist.
- Return `409` with a stable error code such as `project_has_dependencies` when
  retention policy prevents deletion of related email history.
- Revoke all project API credentials atomically with deletion.
- Preserve an immutable administrative audit event containing the delegated
  human actor, project ID, project name, request ID, and timestamp.
- Repeated deletion must be safe and must never delete another project.

MW BFF maps `DELETE /api/mail/projects/{id}` to this exact route and scope, and
requires recent WebAuthn step-up before forwarding the operation.
