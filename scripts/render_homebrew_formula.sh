#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <tag-version> <source-sha256>" >&2
  exit 1
fi

tag_version="$1"
source_sha256="$2"
repo="${GITHUB_REPOSITORY:-disk0Dancer/climate}"

cat <<EOF
class Climate < Formula
  desc "Generate auth-aware Go CLIs from OpenAPI specifications"
  homepage "https://disk0dancer.github.io/climate/"
  url "https://github.com/${repo}/archive/refs/tags/${tag_version}.tar.gz"
  sha256 "${source_sha256}"
  license "Apache-2.0"
  head "https://github.com/${repo}.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/disk0Dancer/climate/cmd/climate/commands.version=${tag_version}"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"climate"), "./cmd/climate"
  end

  test do
    assert_match "climate version v#{version}", shell_output("#{bin}/climate --version")
  end
end
EOF
