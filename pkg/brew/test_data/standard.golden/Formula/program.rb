class Jx < Formula
  desc "A program for helloworld"
  homepage "https://program.io/program/"
  version "1.0.2"

  url "https://github.com/program-org/program/releases/download/v#{version}/program-darwin-amd64.tar.gz"
  sha256 "ef7a95c23bc5858cff6fd2825836af7e8342a9f6821d91ddb0b5b5f87f0f4e85"

  def install
    bin.install name

    output = Utils.popen_read("SHELL=bash #{bin}/program completion bash")
    (bash_completion/"program").write output

    output = Utils.popen_read("SHELL=zsh #{bin}/program completion zsh")
    (zsh_completion/"_program").write output

    prefix.install_metafiles
  end

end