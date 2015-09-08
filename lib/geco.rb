require "geco/version"

require 'json'
require 'open3'
require 'yaml/store'
require 'etc'
require 'singleton'
require 'thor'
require 'text-table'
require 'googleauth'
require 'google/apis/cloudresourcemanager_v1beta1'
require 'google/apis/compute_v1'
require 'thread'

module Geco
  Cloudresourcemanager = ::Google::Apis::CloudresourcemanagerV1beta1
  Compute = ::Google::Apis::ComputeV1

  class Cli < Thor
    class_option :zsh_widget, :type => :boolean, :default => false, :aliases => ['-z']

    option :project, :type => :string, :aliases => ['-p']
    desc "ssh", "select vm instance by peco, and run gcloud compute ssh"
    long_desc <<-LONG_DESC
      show VM instances like `gcloud compute instances list`
      and filter the results by `peco`,
      and execute or print `gcloud compute ssh`
    LONG_DESC
    def ssh
      vm_instance = transaction{
        project = options[:project] || Config.instance.get('core/project')
        VmInstanceList.load(project: project).select(multi:false, project:project)
      }
      if options[:zsh_widget]
        puts vm_instance.build_ssh_cmd
      else
        system vm_instance.build_ssh_cmd
      end
    rescue => e
      if !options[:zsh_widget]
        raise e
      end
    end

    desc "project", "select project and run gcloud config set project"
    long_desc <<-LONG_DESC
      show projects like `gcloud alpha projects list`,
      and filter the results by `peco`,
      and execute or print `gcloud config set project`

      see https://cloud.google.com/sdk/gcloud/reference/alpha/
      about `gcloud alpha ...`
    LONG_DESC
    def project
      project = transaction{ ProjectList.load.select(multi:false) }
      if options[:zsh_widget]
        puts project.build_config_set_project_cmd
      else
        system project.build_config_set_project_cmd
      end
    rescue => e
      if !options[:zsh_widget]
        raise e
      end
    end

    desc "gencache", "cache all projects and instances"
    def gencache
      transaction do
        puts "loading projects..."
        project_list = ProjectList.load(force: true)
        puts "found #{project_list.projects.size} projects."
        mutex = Mutex.new
        threads = []
        project_list.projects.each do |project|
          t = Thread.new do
            mutex.synchronize{ puts "loading project: #{project.name} (#{project.id})" }
            vm_instance_list = VmInstanceList.load(force:true, project: project.id)
            mutex.synchronize{ puts "loaded project: #{project.name} (#{project.id}), found #{vm_instance_list.instances.size} vm instances" }
          end
          threads << t
        end
        threads.each{|t| t.join }
      end
    end

    no_commands do
      def transaction
        Cache.instance.transaction do
          yield
        end
      end
    end
  end

  class Project < Struct.new(:id, :name, :number)
    def build_config_set_project_cmd
      %Q[gcloud config set project "#{id}"]
    end
  end

  ## gcloud alpha projects list
  class ProjectList < Struct.new(:projects)
    def select(multi:false)
      selected = Open3.popen3('peco'){|stdin, stdout, stderr, wait_thr|
        stdin.puts to_table
        stdin.close_write
        stdout.readlines
      }.map{ |line|
        projects.find{|prj| prj.id == line.chomp.gsub(/(^\|\s*|\s*\|$)/, '').split(/\s*\|\s*/).first }
      }.reject{|prj|
        prj.id == "id" || prj.id =~ /^[+-]+$/
      }
      if multi
        selected
      elsif selected.size == 1
        selected.first
      else
        raise "please select 1 Project"
      end
    end

    def to_table
      fields = Project.new.members
      table = Text::Table.new( :head => fields, :rows => projects.map{|i| fields.map{|f| i.send(f)}} )
    end

    class << self
      def load(force:false)
        if force || ! defined?(@@projects)
          @@projects = Cache.instance.get_or_set('projects', expire:24*60*60) do
            service = Cloudresourcemanager::CloudresourcemanagerService.new
            service.authorization = Google::Auth.get_application_default([Cloudresourcemanager::AUTH_CLOUD_PLATFORM])
            service.list_projects.projects.map{|prj|
              Project.new(prj.project_id, prj.name, prj.project_number)
            }
          end
        end
        ProjectList.new(@@projects)
      end
    end
  end

  ## gcloud compute ssh ...
  ## gcloud compute instances ...
  class VmInstance < Struct.new(:project, :name, :zone, :machine_type, :internal_ip, :external_ip, :status)
    def build_ssh_cmd
      %Q[gcloud compute --project="#{project}" ssh --zone="#{zone}" "#{name}"]
    end
  end

  ## gcloud compute instances ...
  class VmInstanceList < Struct.new(:instances)
    def select(multi:false, project:false)
      selected_instances = Open3.popen3('peco'){|stdin, stdout, stderr, wait_thr|
        stdin.puts to_table(with_project: !!project)
        stdin.close_write
        stdout.readlines
      }.map{ |line|
        selected = line.chomp.gsub(/(^\|\s*|\s*\|$)/, '').split(/\s*\|\s*/)
        instances.find{|i| project ? (i.name == selected[0]) : (i.name == selected[1] && i.project == selected[0]) }
      }.reject{|i|
        i.name == "name" || i.name =~ /^[+-]+$/
      }
      if multi
        selected_instances
      elsif selected_instances.size == 1
        selected_instances.first
      else
        raise "please select 1 VM instance"
      end
    end

    def to_table(with_project:false)
      fields = VmInstance.new.members
      fields.reject!{|i| i == :project} if with_project
      table = Text::Table.new( :head => fields, :rows => instances.map{|i| fields.map{|f| i.send(f)}} )
    end

    class << self
      def load(force:false, project:nil)
        if project
          if force || ! defined?(@@instances)
            @@instances = Cache.instance.get_or_set("instances/#{project}") do
              service = Compute::ComputeService.new
              service.authorization = Google::Auth.get_application_default([Compute::AUTH_COMPUTE_READONLY])
              begin
                service.list_aggregated_instances(project).items.values.map(&:instances).flatten.compact.map{|i|
                  VmInstance.new(project, i.name, i.zone.split('/').last, i.machine_type.split('/').last, i.network_interfaces.first.network_ip, i.network_interfaces.first.access_configs.first.nat_ip, i.status)
                }
              rescue Google::Apis::ClientError => e
                warn "ignored exception: #{e.message} (#{e.class})"
                []
              end
            end
          end
          VmInstanceList.new(@@instances)
        else
          project_list = ProjectList.load
          ## TODO: refactoring
          @@instances = project_list.projects.map{|project|
            Cache.instance.get_or_set("instances/#{project.id}") do
              VmInstanceList.load(force:force, project:project.id)
              Cache.instance.get("instances/#{project.id}")
            end
          }.flatten
          VmInstanceList.new(@@instances)
        end
      end
    end
  end

  ## gcloud config ...
  class Config
    include Singleton

    def initialize
      @config = JSON.parse %x[gcloud config list --all --format json]
    end

    def get(arg)
      sec, key = arg.split('/', 2)
      @config[sec][key]
    end
  end

  class Cache
    include Singleton

    def initialize(path:"/tmp/gcloud-cache.#{Etc.getlogin}.yaml")
      @db = YAML::Store.new(path)
    end

    def transaction
      @in_transaction = true
      ret = nil
      @db.transaction do
        ret = yield
      end
    ensure
      @in_transaction = false
      ret
    end

    def get_or_set(key, expire:24*60*60, &block)
      get(key) || set(key, block.call, expire:expire)
    end

    def set(key, value, expire:24*60*60)
      if @in_transaction
        @db[key] = {value: value, expire: Time.now+expire}; value
      else
        self.transaction{ self.set(key, value, expire:expire) }
      end
    end

    def get(key)
      if @in_transaction
        @db[key] && @db[key][:expire] > Time.now ? @db[key][:value] : nil
      else
        self.transaction{ self.get(key) }
      end
    end
  end
end

