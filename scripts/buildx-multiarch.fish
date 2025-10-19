#!/usr/bin/env fish
# Build and optionally push multi-arch DocBuilder images with Buildx.
# Requires: Docker with buildx, and binfmt/qemu for cross-builds (tonistiigi/binfmt).
# Usage:
#   scripts/buildx-multiarch.fish [--push] [--platform linux/amd64,linux/arm64] \
#     [--target runtime-minimal|runtime-full|both] [--registry REG] [--repo OWNER/IMAGE] \
#     [--version VERSION] [--hugo HUGO_VERSION] [--tags TAG1,TAG2]
# Defaults:
#   --platform linux/amd64,linux/arm64
#   --target both
#   --registry (env REGISTRY or empty)
#   --repo (env IMAGE_REPO or "$GITHUB_REPOSITORY")
#   --version (short git sha) and --hugo 0.151.0

function usage
    echo "Usage: scripts/buildx-multiarch.fish [--push] [--platform ...] [--target ...] [--registry REG] [--repo OWNER/IMAGE] [--version VER] [--hugo VER] [--tags TAG1,TAG2]"
end

set -l PUSH 0
set -l PLATFORMS "linux/amd64,linux/arm64"
set -l TARGET "both" # runtime-minimal, runtime-full, both
set -l REG "$REGISTRY"
set -l REPO "$IMAGE_REPO"
if test -z "$REPO"
    if test -n "$GITHUB_REPOSITORY"
        set REPO "$GITHUB_REPOSITORY"
    else
        set REPO "inful/docbuilder"
    end
end
set -l VERSION ""
set -l HUGO_VERSION "0.151.0"
set -l EXTRA_TAGS ""

argparse 'h/help' 'push' 'platform=' 'target=' 'registry=' 'repo=' 'version=' 'hugo=' 'tags=' -- $argv
or begin; usage; exit 2; end

if set -q _flag_help
    usage; exit 0
end
if set -q _flag_push
    set PUSH 1
end
if set -q _flag_platform
    set PLATFORMS $_flag_platform
end
if set -q _flag_target
    set TARGET $_flag_target
end
if set -q _flag_registry
    set REG $_flag_registry
end
if set -q _flag_repo
    set REPO $_flag_repo
end
if set -q _flag_version
    set VERSION $_flag_version
end
if set -q _flag_hugo
    set HUGO_VERSION $_flag_hugo
end
if set -q _flag_tags
    set EXTRA_TAGS $_flag_tags
end

# Compute default VERSION (short sha) if not provided
if test -z "$VERSION"
    if git rev-parse --short=7 HEAD >/dev/null 2>&1
        set VERSION (git rev-parse --short=7 HEAD)
    else
        set VERSION dev
    end
end

# Build full image name base
set -l IMAGE_BASE "$REPO"
if test -n "$REG"
    set IMAGE_BASE "$REG/$IMAGE_BASE"
end

# Ensure buildx is ready
if not docker buildx ls >/dev/null 2>&1
    echo "Enabling buildx builder..."
    docker buildx create --name ci-builder --use >/dev/null
end
# Bootstrap builder
docker buildx inspect --bootstrap >/dev/null

# Prepare tags
set -l BRANCH ""
if test -n "$GITHUB_REF_NAME"
    set BRANCH (string replace -a '/' '-' -- "$GITHUB_REF_NAME")
end
set -l DEFAULT_TAGS "$VERSION"
if test -n "$BRANCH"
    set DEFAULT_TAGS "$DEFAULT_TAGS,$BRANCH"
end
if test -n "$EXTRA_TAGS"
    set DEFAULT_TAGS "$DEFAULT_TAGS,$EXTRA_TAGS"
end

function tag_args
    set -l base $argv[1]
    set -l tags_csv $argv[2]
    set -l args ()
    for t in (string split "," -- $tags_csv)
        set args $args -t "$base:$t"
    end
    printf '%s\n' $args
end

set -l COMMON_ARGS --platform $PLATFORMS --build-arg VERSION=$VERSION --build-arg HUGO_VERSION=$HUGO_VERSION -f Dockerfile .

function build_target
    set -l target $argv[1]
    set -l suffix $argv[2]
    set -l pushflag --load
    if test $PUSH -eq 1
        set pushflag --push
    end
    set -l tags (tag_args $IMAGE_BASE "$DEFAULT_TAGS$suffix")
    echo "Building target=$target platforms=$PLATFORMS tags=$DEFAULT_TAGS$suffix push=$PUSH"
    docker buildx build --target $target $tags $COMMON_ARGS $pushflag
end

switch $TARGET
    case runtime-minimal
        build_target runtime-minimal ""
    case runtime-full
        # add -full suffix to tags
        set DEFAULT_TAGS "$DEFAULT_TAGS-full"
        build_target runtime-full ""
    case both
        build_target runtime-minimal ""
        set DEFAULT_TAGS "$DEFAULT_TAGS-full"
        build_target runtime-full ""
    case '*'
        echo "Unknown target: $TARGET"; exit 1
end

echo "Done. Images based at $IMAGE_BASE"
