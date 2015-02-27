# coding: utf-8
lib = File.expand_path('../lib', __FILE__)
$LOAD_PATH.unshift(lib) unless $LOAD_PATH.include?(lib)
require 'geco/version'

Gem::Specification.new do |spec|
  spec.name          = "geco"
  spec.version       = Geco::VERSION
  spec.authors       = ["YAMADA Tsuyoshi"]
  spec.email         = ["tyamada@minimum2scp.org"]
  spec.summary       = %q{geco = gcloud + peco: select GCP resources using peco, and run gcloud}
  spec.description   = %q{geco = gcloud + peco: select GCP resources using peco, and run gcloud}
  spec.homepage      = "https://github.com/minimum2scp/geco"
  spec.license       = "MIT"

  spec.files         = `git ls-files -z`.split("\x0")
  spec.executables   = spec.files.grep(%r{^bin/}) { |f| File.basename(f) }
  spec.test_files    = spec.files.grep(%r{^(test|spec|features)/})
  spec.require_paths = ["lib"]

  spec.add_runtime_dependency "thor", "~> 0.19.1"
  spec.add_runtime_dependency "text-table", "~> 1.2.4"

  spec.add_development_dependency "bundler", "~> 1.7"
  spec.add_development_dependency "rake", "~> 10.0"
end
