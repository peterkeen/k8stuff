task :setup do
  @TALOS_DIR = File.join(Dir.pwd, "clusters/omicron/talos")

  in_talos_dir do
    validate_tool("op")
    validate_tool("talhelper")

    FileUtils.mkdir_p("clusterconfig")

    sh "op document get --vault fmycvdzmeyvbndk7s7pjyrebtq zjr2jsjcsptwwxjqscu2r4wbze > clusterconfig/talsecret.yaml"
    sh "talhelper genconfig --secret-file clusterconfig/talsecret.yaml --no-gitignore"
    sh "talosctl config use-context dummy"
    sh "talosctl config remove omicron -y"
    sh "talosctl config merge clusterconfig/talosconfig"
    sh "talosctl config use-context omicron"
  end
end

task :apply => :setup do
  cmd = %w[apply]
  if ENV["reboot"] == "true"
    cmd << '--extra-flags="--mode=reboot"'
  end

  talhelper_cmd(cmd.join(" "))
end

task :upgrade => :setup do
  talhelper_cmd("upgrade")
end  

task :dashboard => :setup do
  in_talos_dir do
    exec "talosctl dashboard"
  end
end
