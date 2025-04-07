require './lib/task_common.rb'

Dir.glob('*/**/*.rake') do |f|
  ns = File.dirname(f.to_s).gsub('/', ':')
 
  namespace ns do
    load(f)
  end
end

task :build_all_docker do
  builds = Rake.application.tasks.filter_map do |task|
    task.name if task.name =~ /build_and_push_docker/
  end.to_a

  builds.each do |build|
    Rake::Task[build].invoke
  end
end

task :dump_tasks do
  pp Rake.application.tasks
end
