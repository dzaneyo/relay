export type RecordCategory = 'NOTE' | 'HOST' | 'DATABASE'
export type AuthType = 'NONE' | 'PASSWORD' | 'SSH_KEY'
export type DBType = 'MYSQL' | 'POSTGRESQL' | 'DORIS'

export interface RecordSummary {
  id: string
  name: string
  alias: string
  category: RecordCategory
  notes: string
  favorite: boolean
  tags: string[]
  createdAt: string
  updatedAt: string
}

export interface Credential {
  id?: string
  recordId?: string
  username: string
  authType: AuthType
  secretValue?: string
  keyPath?: string
}

export interface SSHConnection {
  host: string
  port: number
  routeId?: string
}

export interface DatabaseConnection {
  dbType: DBType
  host: string
  port: number
  databaseName?: string
}

export interface RecordDetail {
  record: RecordSummary
  credentials?: Credential[]
  credential?: Credential
  ssh?: SSHConnection
  database?: DatabaseConnection
}

export interface RecordInput {
  name: string
  alias: string
  category: RecordCategory
  notes: string
  favorite: boolean
  tags: string[]
  credential?: Credential
  ssh?: SSHConnection
  database?: DatabaseConnection
}

export interface RouteHop {
  seq: number
  hostRecordId: string
}

export interface SSHRoute {
  id: string
  name: string
  description: string
  hops: RouteHop[]
  createdAt: string
  updatedAt: string
}

export interface RouteInput {
  name: string
  description: string
  hops: RouteHop[]
}
