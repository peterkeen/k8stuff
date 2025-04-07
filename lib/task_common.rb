def build_docker(prereqs: [], repo_base: "ghcr.io/peterkeen/k8stuff", task_dir: nil)
  task_dir ||= File.dirname(caller_locations.first.path)

  build_tag = "#{repo_base}/#{task_dir}:latest"

  task :build_docker => prereqs do
    Dir.chdir(task_dir) do
      sh "docker build -t #{build_tag} ."
    end
  end

  task :push_docker do
    Dir.chdir(task_dir) do
      sh "docker push #{build_tag}"
    end
  end

  task :build_and_push_docker => [:build_docker, :push_docker]
end

def k8s_apply(prereqs: [], context: "admin@omicron", task_dir: nil)
  task_dir ||= File.dirname(caller_locations.first.path)

  task "apply-#{context}" => prereqs do
    validate_tool("kubectl")

    Dir.chdir(task_dir) do
      sh "kubectl --context #{context} apply -f app.yaml"
    end
  end
end

def talhelper_cmd(command)
  validate_tool("talhelper")
  generated = `talhelper gencommand #{command}`
  sh generated
end

def validate_tool(tool)
  unless system("which #{tool}")
    raise "Tool required but not found: #{tool}"
  end
end
