task :apply_repo_secrets => "talos:setup" do
  validate_tool("kubectl")
  validate_tool("op")

  unless system("kubectl get secret ghcr.io")
    secret = `op read "op://Employee/ycvvpvrf7bf4fohratqqyg2eva/personal access tokens/alpine-3-16-docker"`
    sh "kubectl create secret docker-registry ghcr.io --docker-server https://ghcr.io/v2/ --docker-username peterkeen --docker-password #{secret}"
  end
end

task :apply => ["talos:apply", :apply_repo_secrets, "helm:apply"]

