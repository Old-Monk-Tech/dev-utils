class Devkit < Formula
  desc "Developer toolbox (CLI + local API)"
  homepage "https://github.com/yourorg/devkit"
  version "0.1.0"
  # Local file URL to prebuilt binary for this machine
  url "file:///Users/arpandey/Documents/Projects/poc-repos/devkit/dist/devkit"
  sha256 "77a50ec4b482e6a7d3e02d858e6a23ea142831ff4d95f62f26d799904afb43ca"

  def install
    bin.install "devkit"
  end

  service do
    run [opt_bin/"devkit", "start", "--addr", "127.0.0.1:7123"]
    keep_alive true
    log_path var/"log/devkit.log"
    error_log_path var/"log/devkit.err.log"
  end

  test do
    # Test the CLI formatting
    output = shell_output("echo '{\"test\": true}' | #{bin}/devkit format json")
    assert_match "test", output

    # Test help
    assert_match "Developer toolbox", shell_output("#{bin}/devkit --help")
  end
end
