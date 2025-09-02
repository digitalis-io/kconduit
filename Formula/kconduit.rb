# typed: false
# frozen_string_literal: true

class Kconduit < Formula
  desc "Powerful terminal UI for Apache Kafka management"
  homepage "https://github.com/digitalis-io/kconduit"
  version "0.0.1"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/digitalis-io/kconduit/releases/download/v#{version}/kconduit_Darwin_arm64.tar.gz"
      sha256 "" # This will be filled by goreleaser
    end

    if Hardware::CPU.intel?
      url "https://github.com/digitalis-io/kconduit/releases/download/v#{version}/kconduit_Darwin_x86_64.tar.gz"
      sha256 "" # This will be filled by goreleaser
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/digitalis-io/kconduit/releases/download/v#{version}/kconduit_Linux_arm64.tar.gz"
      sha256 "" # This will be filled by goreleaser
    end

    if Hardware::CPU.intel?
      url "https://github.com/digitalis-io/kconduit/releases/download/v#{version}/kconduit_Linux_x86_64.tar.gz"
      sha256 "" # This will be filled by goreleaser
    end
  end

  depends_on :macos => :big_sur if OS.mac?

  def install
    bin.install "kconduit"
    
    # Install shell completions if they exist
    if File.exist?("completions/kconduit.bash")
      bash_completion.install "completions/kconduit.bash"
    end
    
    if File.exist?("completions/kconduit.zsh")
      zsh_completion.install "completions/kconduit.zsh"
    end
    
    if File.exist?("completions/kconduit.fish")
      fish_completion.install "completions/kconduit.fish"
    end
  end

  test do
    assert_match(/kconduit version/, shell_output("#{bin}/kconduit --version"))
  end

  service do
    run [opt_bin/"kconduit"]
    keep_alive false
    working_dir var
    log_path var/"log/kconduit.log"
    error_log_path var/"log/kconduit-error.log"
  end
end