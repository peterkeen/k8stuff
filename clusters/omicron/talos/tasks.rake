task :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    validate_tool("op")
    validate_tool("talhelper")

    sh "op document get --vault fmycvdzmeyvbndk7s7pjyrebtq zjr2jsjcsptwwxjqscu2r4wbze > clusterconfig/talsecret.yaml"
    sh "talhelper genconfig --secret-file clusterconfig/talsecret.yaml --no-gitignore"
    talhelper_cmd("kubeconfig --extra-flags '--force'")
  end
end

task :apply => :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    cmd = %w[apply]
    if ENV["reboot"] == "true"
      cmd << '--extra-args="--mode=reboot"'
    end

    talhelper_cmd(cmd.join(" "))
  end
end

task :upgrade => :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    talhelper_cmd("upgrade")
  end
end  

task :dashboard => :setup do
  validate_tool("taloctl")
  Dir.chdir(File.dirname(__FILE__)) do
    cmd = "talosctl dashboard --talosconfig=./clusterconfig/talosconfig"
    puts cmd
    exec cmd
  end
end
