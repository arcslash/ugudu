# typed: false
# frozen_string_literal: true

# Homebrew formula for Ugudu
# AI Team Orchestration System
class Ugudu < Formula
  desc "AI Team Orchestration System - Create and manage teams of AI agents"
  homepage "https://github.com/arcslash/ugudu"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/arcslash/ugudu/releases/download/v#{version}/ugudu_#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    else
      url "https://github.com/arcslash/ugudu/releases/download/v#{version}/ugudu_#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/arcslash/ugudu/releases/download/v#{version}/ugudu_#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    else
      url "https://github.com/arcslash/ugudu/releases/download/v#{version}/ugudu_#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
  end

  def install
    bin.install "ugudu"
  end

  def caveats
    <<~EOS
      Ugudu has been installed!

      Quick Start:
        1. Configure your API key:
           ugudu config init

        2. Start the daemon:
           ugudu daemon

        3. Create a team:
           ugudu spec new my-team
           ugudu team create alpha --spec my-team

        4. Talk to your team:
           ugudu ask alpha "Hello team!"

      For more information, visit:
        https://github.com/arcslash/ugudu
    EOS
  end

  test do
    assert_match "Ugudu", shell_output("#{bin}/ugudu version")
  end
end
