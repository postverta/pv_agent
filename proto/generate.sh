#!/bin/sh
protoc --go_out=plugins=grpc:. agent.proto
