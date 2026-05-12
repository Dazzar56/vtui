#!/bin/bash
CGO_ENABLED=0 VTUI_DEBUG=1 go run --tags nofakecgo . --gui=gogpu
