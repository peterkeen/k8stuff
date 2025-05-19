def build_docker(prereqs: [], repo_base: "ghcr.io/peterkeen/k8stuff", task_dir: nil, platform: "linux/amd64")  
  task_dir ||= File.dirname(caller_locations.first.path)

  build_tag = "#{repo_base}/#{task_dir}:latest"

  namespace :docker do

    task :build => prereqs do
      Dir.chdir(task_dir) do
        sh "docker build --platform #{platform} -t #{build_tag} ."
      end
    end

    task :push do
      Dir.chdir(task_dir) do
        sh "docker push #{build_tag}"
      end
    end

  end

  task :docker => ["docker:build", "docker:push"]  
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
  in_talos_dir do
    validate_tool("talhelper")
    generated = `talhelper gencommand #{command}`
    sh generated
  end
end

def validate_tool(tool)
  unless system("which #{tool}")
    raise "Tool required but not found: #{tool}"
  end
end

def in_talos_dir
  if @TALOS_DIR.nil?
    warn "Error: you forgot to depend on :setup"
    exit 1
  end

  Dir.chdir(@TALOS_DIR) do
    yield
  end
end
