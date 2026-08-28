# Platform

Reusable platform capabilities belong here: authentication middleware,
observability helpers, generated protobuf code, and cross-service contracts.

Business workflows remain inside their owning service. A platform package must not
import a service module.
