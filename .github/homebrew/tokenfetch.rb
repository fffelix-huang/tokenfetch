# Template for fffelix-huang/homebrew-tap Formula/tokenfetch.rb.
# The release workflow fills in @VERSION@ and @SHA256@ and pushes it to the tap.
class Tokenfetch < Formula
  desc "Fetch-style summary of Claude Code token usage and cost"
  homepage "https://github.com/fffelix-huang/tokenfetch"
  url "https://github.com/fffelix-huang/tokenfetch/archive/refs/tags/v@VERSION@.tar.gz"
  sha256 "@SHA256@"
  license "MIT"
  head "https://github.com/fffelix-huang/tokenfetch.git", branch: "master"

  depends_on "go" => :build

  deny_network_access!

  def fetch
    system "go", "mod", "download"
  end

  def install
    ENV["CGO_ENABLED"] = "0"
    ldflags = %W[
      -s -w
      -X main.version=#{version}
      -X main.revision=#{tap.user}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/tokenfetch"
  end

  test do
    assert_match "#{version} (#{tap.user})", shell_output("#{bin}/tokenfetch --version")
  end
end
