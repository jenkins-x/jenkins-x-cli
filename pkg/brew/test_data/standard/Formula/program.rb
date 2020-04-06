class Jx < Formula
  desc "A program for helloworld"
  homepage "https://program.io/program/"
  version "1.0.1"

  url "https://github.com/program-org/program/releases/download/v#{version}/program-darwin-amd64.tar.gz"
  sha256 "7d7d380c5f0760027ae73f1663a1e1b340548fd93f68956e6b0e2a0d984774fa"

  def install
    bin.install name

    output = Utils.popen_read("SHELL=bash #{bin}/program completion bash")
    (bash_completion/"program").write output

    output = Utils.popen_read("SHELL=zsh #{bin}/program completion zsh")
    (zsh_completion/"_program").write output

    prefix.install_metafiles
  end

end