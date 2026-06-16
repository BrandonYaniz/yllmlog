# Log Processing

When a log is added, yllmlog performs an initial review of the file. The default is to review the whole file, with configurable limits for large or long-lived logs.

The daemon then tracks new lines using stored offsets and file metadata. It detects rotation by checking inode/device changes, file size shrinkage, missing and recreated files, and updated glob targets.

Glob patterns are supported.

The service name should be inferred where possible from the path, filename, process field, or message structure.
