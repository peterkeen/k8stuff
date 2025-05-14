#!/usr/bin/env ruby

require 'json'

class GenerateProxyConf
  CONFIG = <<~HERE
    configVersion: v1
    kubernetes:
    - name: "monitor ingress"
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      executeHookOnEvent: ["Added", "Modified", "Deleted"]
      group: monitors
    - name: "monitor service"
      apiVersion: v1
      kind: Service
      executeHookOnEvent: ["Added", "Modified", "Deleted"]
      group: "monitors"
    EOF
  HERE

  def generate!
    # 1. Collect ingresses of correct ingress class
    # 2. Collect their services
    # 3. Collect their certificates
    # 4. Generate proxy from proxy.conf.erb
    # 5. Build and push context to Minio bucket
    # 6. Run Kamiko pod to build and push oci image to github registry
    # 6. flyctl deploy
  end

  def binding_context
    JSON.parse(File.read(ENV['BINDING_CONTEXT_PATH']))
  end
end


if __FILE__ == $0
  if ARGV[0] == "--config"
    puts GenerateProxyConf::CONFIG
  else
    GenerateProxyConf.new.generate!
  end
end

