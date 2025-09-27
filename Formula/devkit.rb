class Devkit < Formula
  desc "Developer toolbox (CLI + local API)"
  homepage "https://github.com/Old-Monk-Tech/dev-utils"
  license "MIT"

  head "https://github.com/Old-Monk-Tech/dev-utils.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=#{version}", "-o", bin/"devkit", "./cmd/devkit"
  end

  service do
    run [opt_bin/"devkit", "start", "--addr", "127.0.0.1:7123"]
    keep_alive true
    log_path var/"log/devkit.log"
    error_log_path var/"log/devkit.err.log"
  end

  test do
    output = pipe_output("#{bin}/devkit format json", '{"x":1}')
    assert_match '"x": 1', output
    assert_match "Developer toolbox", shell_output("#{bin}/devkit --help")
  end
end

