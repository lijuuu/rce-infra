import { useState, useEffect, useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Select, type SelectOption } from '@/components/ui/select'
import { api, type Node, type Command } from '@/lib/api'
import { 
  RefreshCw, 
  Play, 
  Terminal, 
  Server, 
  LayoutDashboard,
  Activity,
  History,
  HelpCircle,
  Menu,
  X,
  CheckCircle2,
  Clock,
  Search,
  AlertCircle,
  Wifi,
  Cpu,
  HardDrive,
  MemoryStick
} from 'lucide-react'
import { ConsoleView } from '@/components/ConsoleView'

type View = 'dashboard' | 'nodes' | 'commands' | 'history'

function App() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [commands, setCommands] = useState<Command[]>([])
  const [selectedNodeId, setSelectedNodeId] = useState<string>('')
  const [commandInput, setCommandInput] = useState<string>('')
  const [timeoutSec, setTimeoutSec] = useState<number>(30)
  const [loading, setLoading] = useState(false)
  const [currentView, setCurrentView] = useState<View>('dashboard')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [activeConsoleCommand, setActiveConsoleCommand] = useState<Command | null>(null)
  const [nodeSearchQuery, setNodeSearchQuery] = useState<string>('')

  const loadNodes = async () => {
    try {
      const data = await api.listNodes()
      setNodes(data)
      if (data.length > 0 && !selectedNodeId) {
        setSelectedNodeId(data[0].node_id)
      }
    } catch (error) {
      console.error('Failed to load nodes:', error)
    }
  }

  const loadCommands = async () => {
    try {
      const data = await api.listCommands(undefined, 20)
      setCommands(data)
    } catch (error) {
      console.error('Failed to load commands:', error)
    }
  }

  useEffect(() => {
    loadNodes()
    loadCommands()
    const interval = setInterval(() => {
      loadNodes()
      loadCommands()
    }, 5000)
    return () => clearInterval(interval)
  }, [])

  const handleSubmitCommand = async () => {
    if (!selectedNodeId || !commandInput.trim()) {
      alert('Please select a node and enter a command')
      return
    }

    setLoading(true)
    try {
      const result = await api.submitCommand(selectedNodeId, commandInput, timeoutSec)
      setCommandInput('')
      
      // Create a command object for the console view
      const newCommand: Command = {
        command_id: result.command_id,
        node_id: selectedNodeId,
        command_type: 'RunCommand',
        payload: { cmd: commandInput, timeout_sec: timeoutSec },
        status: 'queued',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      
      // Open console view immediately
      setActiveConsoleCommand(newCommand)
      
      // Refresh commands list
      setTimeout(() => {
        loadCommands()
      }, 1000)
    } catch (error: any) {
      alert(`Failed to submit command: ${error.message}`)
    } finally {
      setLoading(false)
    }
  }

  const handleViewLogs = async (commandId: string) => {
    const command = commands.find(c => c.command_id === commandId)
    if (command) {
      setActiveConsoleCommand(command)
    }
  }

  const getStatusBadge = (status: string) => {
    // Map backend statuses to UI-friendly labels
    const statusMap: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "success" | "outline" }> = {
      queued: { label: 'Queued', variant: 'outline' },
      running: { label: 'Live', variant: 'secondary' },
      streaming: { label: 'Live', variant: 'secondary' },
      success: { label: 'Done', variant: 'success' },
      failed: { label: 'Finished', variant: 'destructive' },
      timeout: { label: 'Finished', variant: 'destructive' },
    }
    
    const statusInfo = statusMap[status] || { label: status, variant: 'default' as const }
    return <Badge variant={statusInfo.variant}>{statusInfo.label}</Badge>
  }

  const healthyNodes = nodes.filter(n => n.is_healthy).length
  const totalNodes = nodes.length
  const recentCommands = commands.slice(0, 5)
  const runningCommands = commands.filter(c => c.status === 'running' || c.status === 'streaming').length

  // Helper function to get node display name
  const getNodeDisplayName = (node: Node): string => {
    const attrs = node.attrs || {}
    if (attrs.hostname) return attrs.hostname as string
    if (attrs.ip_address) return attrs.ip_address as string
    return node.node_id.substring(0, 16) + '...'
  }

  // Helper function to get node description
  const getNodeDescription = (node: Node): string => {
    const attrs = node.attrs || {}
    const parts: string[] = []
    if (attrs.ip_address) parts.push(`IP: ${attrs.ip_address}`)
    if (attrs.os_name) parts.push(attrs.os_name as string)
    if (attrs.arch) parts.push(attrs.arch as string)
    return parts.length > 0 ? parts.join(' • ') : node.node_id
  }

  // Create select options for nodes
  const nodeSelectOptions: SelectOption[] = useMemo(() => {
    return nodes.map(node => ({
      value: node.node_id,
      label: getNodeDisplayName(node),
      description: getNodeDescription(node),
      disabled: node.disabled || !node.is_healthy,
    }))
  }, [nodes])

  // Filter nodes for display
  const filteredNodes = useMemo(() => {
    if (!nodeSearchQuery) return nodes
    const query = nodeSearchQuery.toLowerCase()
    return nodes.filter(node => {
      const displayName = getNodeDisplayName(node).toLowerCase()
      const description = getNodeDescription(node).toLowerCase()
      const nodeId = node.node_id.toLowerCase()
      return displayName.includes(query) || description.includes(query) || nodeId.includes(query)
    })
  }, [nodes, nodeSearchQuery])

  const sidebarItems = [
    { id: 'dashboard' as View, label: 'Dashboard', icon: LayoutDashboard },
    { id: 'nodes' as View, label: 'Nodes', icon: Server },
    { id: 'commands' as View, label: 'Commands', icon: Terminal },
    { id: 'history' as View, label: 'History', icon: History },
  ]

  return (
    <div className="flex h-screen bg-gray-50 overflow-hidden">
      {/* Sidebar */}
      <aside className={`${sidebarOpen ? 'w-64' : 'w-0'} transition-all duration-300 bg-white border-r border-gray-200 flex flex-col overflow-hidden`}>
        <div className="p-6 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
                <Terminal className="w-5 h-5 text-white" />
              </div>
              <span className="font-semibold text-lg">Command Exec</span>
            </div>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="md:hidden"
            >
              <X className="w-5 h-5" />
            </Button>
          </div>
        </div>
        
        <nav className="flex-1 p-4">
          <div className="space-y-1">
            {sidebarItems.map((item) => {
              const Icon = item.icon
              return (
                <button
                  key={item.id}
                  onClick={() => setCurrentView(item.id)}
                  className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors ${
                    currentView === item.id
                      ? 'bg-primary text-white'
                      : 'text-gray-700 hover:bg-gray-100'
                  }`}
                >
                  <Icon className="w-5 h-5" />
                  <span className="font-medium">{item.label}</span>
                </button>
              )
            })}
          </div>
        </nav>
      </aside>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <header className="bg-white border-b border-gray-200 px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setSidebarOpen(!sidebarOpen)}
                className="md:hidden"
              >
                <Menu className="w-5 h-5" />
              </Button>
              <div>
                <h1 className="text-xl font-semibold text-gray-900">
                  {currentView === 'dashboard' && 'Dashboard'}
                  {currentView === 'nodes' && 'Nodes'}
                  {currentView === 'commands' && 'Execute Commands'}
                  {currentView === 'history' && 'Command History'}
                </h1>
              </div>
            </div>
            <div className="flex items-center gap-4">
              <Button 
                className="bg-primary hover:bg-primary/90"
                onClick={() => {
                  loadNodes()
                  loadCommands()
                }}
              >
                Refresh
                <RefreshCw className="w-4 h-4 ml-2" />
              </Button>
            </div>
          </div>
        </header>

        {/* Content Area */}
        <main className="flex-1 overflow-y-auto p-6">
          {currentView === 'dashboard' && (
            <div className="space-y-6">
              {/* Stats Cards */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <Card>
                  <CardContent className="p-6">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-gray-600">Total Nodes</p>
                        <p className="text-3xl font-bold text-gray-900 mt-2">{totalNodes}</p>
                      </div>
                      <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                        <Server className="w-6 h-6 text-blue-600" />
                      </div>
                    </div>
                  </CardContent>
                </Card>
                
                <Card>
                  <CardContent className="p-6">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-gray-600">Healthy Nodes</p>
                        <p className="text-3xl font-bold text-green-600 mt-2">{healthyNodes}</p>
                      </div>
                      <div className="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center">
                        <CheckCircle2 className="w-6 h-6 text-green-600" />
                      </div>
                    </div>
                  </CardContent>
                </Card>
                
                <Card>
                  <CardContent className="p-6">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-gray-600">Running Commands</p>
                        <p className="text-3xl font-bold text-orange-600 mt-2">{runningCommands}</p>
                      </div>
                      <div className="w-12 h-12 bg-orange-100 rounded-lg flex items-center justify-center">
                        <Activity className="w-6 h-6 text-orange-600" />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>

              {/* Recent Activity */}
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <Card>
                  <CardHeader>
                    <CardTitle>Recent Commands</CardTitle>
                    <CardDescription>Latest command executions</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-3">
                      {recentCommands.length === 0 ? (
                        <div className="text-center py-8">
                          <Terminal className="w-8 h-8 text-gray-300 mx-auto mb-2" />
                          <p className="text-sm text-gray-500">No commands yet</p>
                        </div>
                      ) : (
                        recentCommands.map((cmd) => {
                          const node = nodes.find(n => n.node_id === cmd.node_id)
                          return (
                            <div 
                              key={cmd.command_id} 
                              className="flex items-center justify-between p-3 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
                              onClick={() => handleViewLogs(cmd.command_id)}
                            >
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2 mb-1 flex-wrap">
                                  <code className="text-xs font-mono text-gray-700 truncate max-w-[200px]">
                                    {(cmd.payload as any)?.cmd?.substring(0, 40) || 'N/A'}
                                    {(cmd.payload as any)?.cmd?.length > 40 && '...'}
                                  </code>
                                  {getStatusBadge(cmd.status)}
                                </div>
                                <div className="text-xs text-gray-500 flex items-center gap-2">
                                  {node ? (
                                    <>
                                      <span>{getNodeDisplayName(node)}</span>
                                      <span>•</span>
                                    </>
                                  ) : (
                                    <>
                                      <span>{cmd.node_id.substring(0, 8)}...</span>
                                      <span>•</span>
                                    </>
                                  )}
                                  <span>{new Date(cmd.created_at).toLocaleString()}</span>
                                </div>
                              </div>
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  handleViewLogs(cmd.command_id)
                                }}
                                className="shrink-0 ml-2"
                              >
                                View
                              </Button>
                            </div>
                          )
                        })
                      )}
                    </div>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>Node Status</CardTitle>
                    <CardDescription>Current node health overview</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2">
                      {nodes.slice(0, 5).map((node) => (
                        <div 
                          key={node.node_id} 
                          className="flex items-center justify-between p-3 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
                          onClick={() => {
                            setSelectedNodeId(node.node_id)
                            setCurrentView('commands')
                          }}
                        >
                          <div className="flex items-center gap-3 flex-1 min-w-0">
                            <div className={`w-2 h-2 rounded-full shrink-0 ${node.is_healthy ? 'bg-green-500' : 'bg-red-500'}`} />
                            <div className="flex-1 min-w-0">
                              <div className="font-medium text-sm truncate">{getNodeDisplayName(node)}</div>
                              <div className="text-xs text-gray-500 truncate">{getNodeDescription(node)}</div>
                            </div>
                          </div>
                          <Badge variant={node.is_healthy ? 'success' : 'destructive'} className="shrink-0 ml-2">
                            {node.is_healthy ? 'Healthy' : 'Unhealthy'}
                          </Badge>
                        </div>
                      ))}
                      {nodes.length === 0 && (
                        <p className="text-center text-gray-500 py-4">No nodes registered</p>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
          )}

          {currentView === 'nodes' && (
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle>All Nodes</CardTitle>
                      <CardDescription>Manage and monitor all registered nodes</CardDescription>
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="relative">
                        <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
                        <Input
                          placeholder="Search nodes..."
                          value={nodeSearchQuery}
                          onChange={(e) => setNodeSearchQuery(e.target.value)}
                          className="pl-9 w-64"
                        />
                      </div>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    {filteredNodes.length === 0 ? (
                      <div className="text-center py-12">
                        <Server className="w-12 h-12 text-gray-300 mx-auto mb-4" />
                        <p className="text-gray-500">
                          {nodeSearchQuery ? 'No nodes found matching your search' : 'No nodes registered'}
                        </p>
                      </div>
                    ) : (
                      filteredNodes.map((node) => {
                        const attrs = node.attrs || {}
                        return (
                          <div
                            key={node.node_id}
                            className={`p-4 border rounded-lg cursor-pointer transition-all ${
                              selectedNodeId === node.node_id
                                ? 'border-primary bg-primary/5 shadow-sm'
                                : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50 hover:shadow-sm'
                            }`}
                            onClick={() => setSelectedNodeId(node.node_id)}
                          >
                            <div className="flex items-start justify-between gap-4">
                              <div className="flex items-start gap-3 flex-1 min-w-0">
                                <div className={`w-3 h-3 rounded-full mt-1.5 shrink-0 ${node.is_healthy ? 'bg-green-500' : 'bg-red-500'}`} />
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center gap-2 mb-1">
                                    <div className="font-semibold text-gray-900 truncate">{getNodeDisplayName(node)}</div>
                                    {node.disabled && (
                                      <Badge variant="outline" className="text-xs">Disabled</Badge>
                                    )}
                                  </div>
                                  <div className="text-xs text-gray-500 mb-2">
                                    <code className="bg-gray-100 px-1.5 py-0.5 rounded">{node.node_id}</code>
                                  </div>
                                  <div className="flex flex-wrap gap-4 text-xs text-gray-600">
                                    {attrs.hostname && (
                                      <div className="flex items-center gap-1">
                                        <Server className="w-3 h-3" />
                                        <span>{attrs.hostname as string}</span>
                                      </div>
                                    )}
                                    {attrs.ip_address && (
                                      <div className="flex items-center gap-1">
                                        <Wifi className="w-3 h-3" />
                                        <span>{attrs.ip_address as string}</span>
                                      </div>
                                    )}
                                    {attrs.os_name && (
                                      <div className="flex items-center gap-1">
                                        <span>{attrs.os_name as string}</span>
                                        {attrs.arch && <span className="text-gray-400">({attrs.arch as string})</span>}
                                      </div>
                                    )}
                                    {attrs.cpu_cores && (
                                      <div className="flex items-center gap-1">
                                        <Cpu className="w-3 h-3" />
                                        <span>{attrs.cpu_cores as number} cores</span>
                                      </div>
                                    )}
                                    {attrs.memory_mb && (
                                      <div className="flex items-center gap-1">
                                        <MemoryStick className="w-3 h-3" />
                                        <span>{Math.round((attrs.memory_mb as number) / 1024)}GB RAM</span>
                                      </div>
                                    )}
                                    {attrs.disk_gb && (
                                      <div className="flex items-center gap-1">
                                        <HardDrive className="w-3 h-3" />
                                        <span>{attrs.disk_gb as number}GB disk</span>
                                      </div>
                                    )}
                                  </div>
                                  <div className="text-xs text-gray-500 flex items-center gap-1 mt-2">
                                    <Clock className="w-3 h-3" />
                                    Last seen: {new Date(node.last_seen_at).toLocaleString()}
                                  </div>
                                </div>
                              </div>
                              <div className="flex flex-col items-end gap-2 shrink-0">
                                <Badge variant={node.is_healthy ? 'success' : 'destructive'}>
                                  {node.is_healthy ? (
                                    <>
                                      <CheckCircle2 className="w-3 h-3 mr-1" />
                                      Healthy
                                    </>
                                  ) : (
                                    <>
                                      <AlertCircle className="w-3 h-3 mr-1" />
                                      Unhealthy
                                    </>
                                  )}
                                </Badge>
                                {selectedNodeId === node.node_id && (
                                  <Badge variant="default" className="text-xs">
                                    Selected
                                  </Badge>
                                )}
                              </div>
                            </div>
                          </div>
                        )
                      })
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>
          )}

          {currentView === 'commands' && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <Card>
                <CardHeader>
                  <CardTitle>Execute Command</CardTitle>
                  <CardDescription>Run a command on the selected node</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="node-select">Select Node</Label>
                    {nodes.length === 0 ? (
                      <div className="p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
                        <div className="flex items-center gap-2 text-yellow-800">
                          <AlertCircle className="w-4 h-4" />
                          <span className="text-sm">No nodes available. Please register a node first.</span>
                        </div>
                      </div>
                    ) : (
                      <Select
                        options={nodeSelectOptions}
                        value={selectedNodeId}
                        onValueChange={setSelectedNodeId}
                        placeholder="Select a node..."
                        searchable={true}
                      />
                    )}
                    {selectedNodeId && (
                      <div className="p-3 bg-gray-50 rounded-lg border border-gray-200">
                        <div className="flex items-center gap-2 mb-1">
                          {nodes.find(n => n.node_id === selectedNodeId)?.is_healthy ? (
                            <CheckCircle2 className="w-4 h-4 text-green-600" />
                          ) : (
                            <AlertCircle className="w-4 h-4 text-red-600" />
                          )}
                          <span className="font-medium text-sm">
                            {nodes.find(n => n.node_id === selectedNodeId) 
                              ? getNodeDisplayName(nodes.find(n => n.node_id === selectedNodeId)!)
                              : selectedNodeId}
                          </span>
                        </div>
                        {nodes.find(n => n.node_id === selectedNodeId) && (
                          <div className="text-xs text-gray-500 mt-1">
                            {getNodeDescription(nodes.find(n => n.node_id === selectedNodeId)!)}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="command">Command</Label>
                    <Input
                      id="command"
                      placeholder="e.g., echo 'Hello World' && date"
                      value={commandInput}
                      onChange={(e) => setCommandInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && e.ctrlKey) {
                          handleSubmitCommand()
                        }
                      }}
                      className="font-mono text-sm"
                    />
                    <p className="text-xs text-gray-500">Press Ctrl+Enter to execute</p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="timeout">Timeout (seconds)</Label>
                    <Input
                      id="timeout"
                      type="number"
                      value={timeoutSec}
                      onChange={(e) => setTimeoutSec(parseInt(e.target.value) || 30)}
                      min="1"
                      max="300"
                    />
                    <p className="text-xs text-gray-500">Maximum execution time (1-300 seconds)</p>
                  </div>
                  <Button
                    onClick={handleSubmitCommand}
                    disabled={loading || !selectedNodeId || !commandInput.trim()}
                    className="w-full bg-primary hover:bg-primary/90"
                  >
                    {loading ? (
                      <>
                        <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                        Submitting...
                      </>
                    ) : (
                      <>
                        <Play className="w-4 h-4 mr-2" />
                        Execute Command
                      </>
                    )}
                  </Button>
                  {!selectedNodeId && (
                    <div className="p-3 bg-blue-50 border border-blue-200 rounded-lg">
                      <div className="flex items-center gap-2 text-blue-800">
                        <HelpCircle className="w-4 h-4" />
                        <span className="text-sm">Select a node from the dropdown above to execute commands</span>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Quick Actions</CardTitle>
                  <CardDescription>Common commands and shortcuts</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    {[
                      { label: 'System Info', cmd: 'uname -a && uptime' },
                      { label: 'Disk Usage', cmd: 'df -h' },
                      { label: 'Memory Info', cmd: 'free -h' },
                      { label: 'Network Stats', cmd: 'netstat -i' },
                    ].map((action) => (
                      <Button
                        key={action.label}
                        variant="outline"
                        className="w-full justify-start"
                        onClick={() => setCommandInput(action.cmd)}
                      >
                        <Terminal className="w-4 h-4 mr-2" />
                        {action.label}
                      </Button>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </div>
          )}

          {currentView === 'history' && (
            <Card>
              <CardHeader>
                <CardTitle>Command History</CardTitle>
                <CardDescription>View all command executions and their logs</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {commands.length === 0 ? (
                    <div className="text-center py-12">
                      <Terminal className="w-12 h-12 text-gray-300 mx-auto mb-4" />
                      <p className="text-gray-500">No commands executed yet</p>
                      <p className="text-sm text-gray-400 mt-2">Execute a command from the Commands tab to see history</p>
                    </div>
                  ) : (
                    commands.map((cmd) => {
                      const node = nodes.find(n => n.node_id === cmd.node_id)
                      return (
                        <div
                          key={cmd.command_id}
                          className="p-4 border border-gray-200 rounded-lg hover:bg-gray-50 hover:shadow-sm transition-all cursor-pointer"
                          onClick={() => handleViewLogs(cmd.command_id)}
                        >
                          <div className="flex items-start justify-between mb-3">
                            <div className="flex items-center gap-2 flex-1 min-w-0">
                              <div className="flex items-center gap-2 min-w-0">
                                {getStatusBadge(cmd.status)}
                                <span className="font-mono text-xs text-gray-500 truncate">
                                  {cmd.command_id.substring(0, 16)}...
                                </span>
                              </div>
                            </div>
                            <div className="text-xs text-gray-500 shrink-0 ml-2">
                              {new Date(cmd.created_at).toLocaleString()}
                            </div>
                          </div>
                          <div className="space-y-2 mb-3">
                            <div className="flex items-center gap-2 text-sm">
                              <span className="text-gray-600">Node:</span>
                              <div className="flex items-center gap-1">
                                {node && (
                                  <>
                                    {node.is_healthy ? (
                                      <CheckCircle2 className="w-3 h-3 text-green-600" />
                                    ) : (
                                      <AlertCircle className="w-3 h-3 text-red-600" />
                                    )}
                                    <span className="font-medium">{getNodeDisplayName(node)}</span>
                                  </>
                                )}
                                {!node && (
                                  <span className="font-medium text-gray-500">{cmd.node_id.substring(0, 16)}...</span>
                                )}
                              </div>
                            </div>
                            <div className="text-sm">
                              <span className="text-gray-600">Command:</span>
                              <div className="mt-1">
                                <code className="bg-gray-100 px-2 py-1 rounded text-xs font-mono block break-all">
                                  {(cmd.payload as any)?.cmd || 'N/A'}
                                </code>
                              </div>
                            </div>
                            {cmd.status === 'success' && cmd.exit_code !== undefined && (
                              <div className="flex items-center gap-2 text-xs">
                                <CheckCircle2 className="w-3 h-3 text-green-600" />
                                <span className="text-gray-600">Exit code: <span className="font-medium">{cmd.exit_code}</span></span>
                              </div>
                            )}
                            {(cmd.status === 'failed' || cmd.status === 'timeout') && cmd.error_msg && (
                              <div className="text-xs text-red-600 bg-red-50 p-2 rounded border border-red-200">
                                <div className="flex items-center gap-1 mb-1">
                                  <AlertCircle className="w-3 h-3" />
                                  <span className="font-medium">Error:</span>
                                </div>
                                <div className="pl-4">{cmd.error_msg}</div>
                              </div>
                            )}
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleViewLogs(cmd.command_id)
                            }}
                            className="w-full sm:w-auto"
                          >
                            <Terminal className="w-3 h-3 mr-2" />
                            View Logs
                          </Button>
                        </div>
                      )
                    })
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Real-time Console View */}
          {activeConsoleCommand && (
            <ConsoleView
              command={activeConsoleCommand}
              onClose={() => {
                setActiveConsoleCommand(null)
                // Refresh commands to get updated status
                loadCommands()
              }}
              onCommandUpdate={(updatedCommand) => {
                // Update the command in the commands list
                setCommands(prev => 
                  prev.map(cmd => 
                    cmd.command_id === updatedCommand.command_id ? updatedCommand : cmd
                  )
                )
              }}
            />
          )}
        </main>
      </div>
      </div>
  )
}

export default App
