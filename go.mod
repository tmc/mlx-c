module github.com/tmc/mlx-c

go 1.25.0

require gopkg.in/yaml.v3 v3.0.1

require github.com/ebitengine/purego v0.10.1

// Pinned to the apple-rdma branch (ahead of v0.6.13): the tip carries
// IbvModifyQpToErr / IBV_QPS_ERR, which no semver tag includes yet. Bump to a
// tag once one is cut on that line.
require github.com/tmc/apple v0.6.14-0.20260702224626-3bb6555763ee
