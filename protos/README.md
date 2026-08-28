# Protobuf APIs

This directory is the source of truth for versioned protobuf definitions. APIs will
follow the Google AIP resource-oriented model, including canonical resource names,
standard methods where applicable, versioned packages, and lower_snake_case fields.

Generated files belong under the appropriate `platform` output directory and must
not be edited manually. The Buf/protoc generation configuration will be introduced
with the first account contract.
