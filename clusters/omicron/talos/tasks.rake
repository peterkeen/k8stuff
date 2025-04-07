task :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    sh "op document get --vault fmycvdzmeyvbndk7s7pjyrebtq zjr2jsjcsptwwxjqscu2r4wbze > clusterconfig/talsecret.yaml"
    sh "talhelper genconfig --secret-file clusterconfig/talsecret.yaml --no-gitignore"      
    sh "talhelper gencommand kubeconfig --extra-flags '--force' | bash"
  end
end

task :apply => :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    cmd = %w[talhelper gencommand apply]
    if ENV["reboot"] == "true"
      cmd << '--extra-args="--mode=reboot"'
    end

    sh "#{cmd.join(" ")} | bash"
  end
end

task :upgrade => :setup do
  Dir.chdir(File.dirname(__FILE__)) do
    cmd = %w[talhelper gencommand upgrade]

    sh "#{cmd.join(" ")} | bash"
  end
end  
