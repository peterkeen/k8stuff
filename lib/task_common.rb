def build_docker(prereqs: [], repo_base: "ghcr.io/keenfamily-us/infra", task_dir: nil)
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
    Dir.chdir(task_dir) do
      sh "kubectl --context #{context} apply -f app.yaml"
    end
  end
end
