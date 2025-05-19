task :setup => "clusters:omicron:talos:setup" do
  validate_tool("op")

  Dir.chdir(File.dirname(__FILE__)) do
    unless File.exist?(".secrets/1password-token")
      sh "op connect token create Kubernetes --server omicron --vault fmycvdzmeyvbndk7s7pjyrebtq > .secrets/1password-token"
    end

    unless File.exist?(".secrets/1password-credentials.json")
      sh "op document get --vault fmycvdzmeyvbndk7s7pjyrebtq zuachmxoynq6upfbusnj55e6u4 > .secrets/1password-credentials.json"
    end
  end
end

task :apply => :setup do
  validate_tool("op")
  validate_tool("helmfile")

  Dir.chdir(File.dirname(__FILE__)) do
    sh "helmfile init"
    sh "op inject -i helmfile.yaml | helmfile apply -f -"
  end
end

task :preview => :setup do
  validate_tool("op")
  validate_tool("helmfile")

  Dir.chdir(File.dirname(__FILE__)) do
    sh "helmfile init"
    sh "op inject -i helmfile.yaml | helmfile diff -f -"
  end
end
