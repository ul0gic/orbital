# typed: false
# frozen_string_literal: true

class Orbital < Formula
  desc "Ephemeral two-person file exchange over a Cloudflare quick tunnel"
  homepage "https://github.com/ul0gic/orbital"
  version "0.0.0"
  license "MIT"

  depends_on "cloudflared"

  on_macos do
    on_arm do
      url "https://github.com/ul0gic/orbital/releases/download/v#{version}/orbital_#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_AT_RELEASE_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/ul0gic/orbital/releases/download/v#{version}/orbital_#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_AT_RELEASE_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/ul0gic/orbital/releases/download/v#{version}/orbital_#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_AT_RELEASE_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/ul0gic/orbital/releases/download/v#{version}/orbital_#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_AT_RELEASE_LINUX_AMD64"
    end
  end

  def install
    bin.install "orbital"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/orbital --version")
  end
end
