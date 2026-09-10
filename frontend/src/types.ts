export type CurrentUser = {
  name?: string;
  avatar?: string;
  plugins?: Array<{
    path: string;
    name: string;
    create_at?: string;
    type?: "node" | "python" | string;
    file?: string;
    plugin?: string;
  }>;
  adapters?: AdapterStatus[];
  integrations?: Record<string, IntegrationStatus>;
  user_stats?: UserStats;
  version?: VersionInfo;
};

export type AdapterStatus = {
  platform: string;
  label: string;
  online: boolean;
  enabled?: boolean;
  manageable?: boolean;
  bots_id?: string[];
  count?: number;
};

export type IntegrationStatus = {
  label: string;
  count: number;
  online_count: number;
  online: boolean;
};

export type VersionInfo = {
  local?: string;
  remote?: string;
  source?: string;
  repository?: string;
};

export type UserStats = {
  total?: number;
  today?: number;
  error?: string;
};

export type AdminUserRow = {
  id: string;
  username: string;
  nickname: string;
  created_at?: number;
  updated_at?: number;
  disabled?: boolean;
  storage_key?: string;
  bindings?: {
    qq?: string;
    telegram?: string;
    updated_at?: number;
  };
};

export type PluginInfo = {
  id: string;
  title: string;
  admin?: boolean;
  cron?: Record<string, string>;
  type?: string;
  suffix?: string;
  desc?: string;
  rule?: string;
  version?: string;
  author?: string;
  icon?: string;
  status?: boolean;
  install_status?: number;
  current_version?: string;
  latest_version?: string;
  update_content?: string;
  running?: boolean;
  debug?: boolean;
  public?: boolean;
  open?: boolean;
  has_form?: boolean;
  has_user_form?: boolean;
  config_registered?: boolean;
  module?: boolean;
  on_start?: boolean;
  create_at?: string;
  class?: string;
  organization?: string;
  address?: string;
  messages?: unknown;
  dependencies?: string[];
  module_dependencies?: string[];
  missing_module_dependencies?: string[];
};

export type PluginSourceInfo = {
  address: string;
  disabled: boolean;
};

export type AppScriptInfo = {
  id: string;
  title: string;
  name: string;
  language: string;
  executable: boolean;
  file: string;
  desc?: string;
  version?: string;
  status?: boolean;
  on_start?: boolean;
  web?: boolean;
  has_cron?: boolean;
  has_form?: boolean;
  installed?: boolean;
  create_at?: string;
};

export type Reply = {
  id?: number;
  index?: number;
  nickname?: string;
  number?: string;
  priority?: number;
  keyword?: string;
  value?: string;
  created_at?: number;
  platforms?: string[];
  enable?: boolean | null;
};

export type Master = {
  id?: number;
  platform?: string;
  nickname?: string;
  number?: string;
  unix?: number;
};

export type CarryTarget = {
  platform: string;
  chat_id: string;
  type?: "group" | "private";
};

export type CarryGroup = {
  id?: number;
  chat_id: string;
  remark?: string;
  platform?: string;
  created_at?: number;
  bots_id?: string[];
  scripts?: string[];
  targets?: CarryTarget[];
  enable?: boolean;
};

export type Task = {
  id?: number;
  task_id?: string;
  title?: string;
  schedule?: string;
  senders?: Array<{
    chat_id?: string;
    user_id?: string;
    platform?: string;
    bot_id?: string;
  }>;
  command?: string;
  trigger?: string;
  scripts?: string[];
  created_at?: number;
  remark?: string;
  enable?: boolean;
};

export type QinglongPanel = {
  id?: string;
  name?: string;
  address: string;
  client_id: string;
  client_secret: string;
  created_at?: number;
  updated_at?: number;
  last_checked_at?: number;
  status?: string;
  message?: string;
};

export type DaidaiPanel = {
  id?: string;
  name?: string;
  address: string;
  app_key: string;
  app_secret: string;
  created_at?: number;
  updated_at?: number;
  last_checked_at?: number;
  status?: string;
  message?: string;
};
