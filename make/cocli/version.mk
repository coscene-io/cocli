BUILD_DATE            := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT            := $(shell git rev-parse HEAD)
GIT_REMOTE            := origin
GIT_BRANCH            := $(shell git rev-parse --symbolic-full-name --verify --quiet --abbrev-ref HEAD)
GIT_TAG               := $(shell git describe --exact-match --tags --abbrev=0  2> /dev/null || echo untagged)
GIT_TREE_STATE        := $(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)
RELEASE_TAG           := $(shell if [[ "$(GIT_TAG)" =~ ^v[0-9]+\.[0-9]+\.[0-9]+.*$$ ]]; then echo "true"; else echo "false"; fi)
VERSION               := latest

# VERSION is the version to be used for files in manifests and should always be latest unless we are releasing
# we assume HEAD means you are on a tag
ifeq ($(RELEASE_TAG),true)
VERSION               := $(GIT_TAG)
endif

override LDFLAGS += \
  -X github.com/coscene-io/cocli.version=$(VERSION) \
  -X github.com/coscene-io/cocli.gitCommit=${GIT_COMMIT} \
  -X github.com/coscene-io/cocli.gitTreeState=${GIT_TREE_STATE}

ifneq ($(GIT_TAG),)
override LDFLAGS += -X github.com/coscene-io/cocli.gitTag=${GIT_TAG}
endif