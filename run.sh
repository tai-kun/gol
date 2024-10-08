#!/usr/bin/env bash

# Terminal: 1
# $ surreal start -u root -p root -l debug

# Terminal: 2
# $ surreal sql -u root -p root --pretty
# > USE NS test
# > DEFINE USER logger ON NAMESPACE PASSWORD s'iamlogger' ROLES EDITOR

# Terminal: 3
# $ bash ./run.sh bash ./proc.sh

# Terminal: 2
# > INFO FOR NS
# > USE DB <database>
# > INFO FOR DB
# > SELECT * FROM <table> ORDER BY time

export GOL_SURREAL_HOST='localhost:8000'
export GOL_SURREAL_USER='logger'
export GOL_SURREAL_PASS='iamlogger'
export GOL_SURREAL_NS='test'

go run ./main.go $*
