import { useEffect, useRef, useState, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { api, type Command, type LogChunk } from '@/lib/api'
import { X, RefreshCw, Square } from 'lucide-react'

interface ConsoleViewProps {
  command: Command
  onClose: () => void
  onCommandUpdate?: (command: Command) => void
}

export function ConsoleView({ command, onClose }: ConsoleViewProps) {
  const [logs, setLogs] = useState<LogChunk[]>([])
  const [commandStatus, setCommandStatus] = useState<string>(command.status)
  const [currentCommand, setCurrentCommand] = useState<Command>(command)
  const [hasFinalChunk, setHasFinalChunk] = useState<boolean>(false)
  const consoleEndRef = useRef<HTMLDivElement>(null)
  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const pollingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const lastChunkIndexRef = useRef<number>(-1)
  const shouldPollRef = useRef<boolean>(true)

  // Update status when command prop changes (from parent)
  useEffect(() => {
    setCommandStatus(command.status)
    setCurrentCommand(command)
    
    // If command_id changes, reset state
    if (command.command_id !== currentCommand.command_id) {
      setHasFinalChunk(false)
      lastChunkIndexRef.current = -1
      shouldPollRef.current = true
    }
    
    // Stop polling immediately if command is finished or timed out
    if (command.status === 'success' || command.status === 'failed' || command.status === 'timeout') {
      shouldPollRef.current = false
      // Clear polling interval immediately
      if (pollingIntervalRef.current) {
        clearInterval(pollingIntervalRef.current)
        pollingIntervalRef.current = null
      }
    }
  }, [command.command_id, command.status, currentCommand.command_id])

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (consoleEndRef.current && scrollContainerRef.current) {
      const container = scrollContainerRef.current
      const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 100
      if (isNearBottom) {
        consoleEndRef.current.scrollIntoView({ behavior: 'smooth' })
      }
    }
  }, [logs])

  // Fetch logs function - checks is_final on every chunk
  const fetchLogs = useCallback(async () => {
    // Stop if polling is disabled, timeout reached, or command is finished
    if (!shouldPollRef.current || 
        commandStatus === 'timeout' || 
        commandStatus === 'success' || 
        commandStatus === 'failed') {
      // Stop polling immediately if timeout or finished
      if (commandStatus === 'timeout' || commandStatus === 'success' || commandStatus === 'failed') {
        shouldPollRef.current = false
        if (pollingIntervalRef.current) {
          clearInterval(pollingIntervalRef.current)
          pollingIntervalRef.current = null
        }
      }
      return
    }

    try {
      const currentIndex = lastChunkIndexRef.current
      const url = currentIndex >= 0
        ? `http://localhost:8080/v1/commands/${command.command_id}/logs?after_chunk_index=${currentIndex}`
        : `http://localhost:8080/v1/commands/${command.command_id}/logs`
      
      const response = await fetch(url)
      if (!response.ok) return
      
      const data = await response.json()
      if (data?.logs && Array.isArray(data.logs) && data.logs.length > 0) {
        // Sort by chunk_index and add new logs
        const newLogs = data.logs.sort((a: LogChunk, b: LogChunk) => a.chunk_index - b.chunk_index)
        
        // Check EVERY chunk for is_final field - if any chunk is final, stop polling
        const hasFinal = newLogs.some((log: LogChunk) => log.is_final === true)
        if (hasFinal) {
          setHasFinalChunk(true)
          shouldPollRef.current = false
          // Stop polling immediately
          if (pollingIntervalRef.current) {
            clearInterval(pollingIntervalRef.current)
            pollingIntervalRef.current = null
          }
        }
        
        setLogs(prev => {
          const existing = new Map(prev.map(log => [`${log.chunk_index}-${log.stream}`, log]))
          newLogs.forEach((log: LogChunk) => {
            existing.set(`${log.chunk_index}-${log.stream}`, log)
          })
          const sortedLogs = Array.from(existing.values()).sort((a, b) => {
            // System messages (chunk_index -1) always go last
            if (a.chunk_index === -1) return 1
            if (b.chunk_index === -1) return -1
            return a.chunk_index - b.chunk_index
          })
          
          // Add "Work Done" message to console when final chunks are received
          if (hasFinal && !prev.some(log => log.chunk_index === -1 && log.stream === 'system')) {
            // Add system message at the end (it will be sorted to the end)
            sortedLogs.push({
              chunk_index: -1,
              stream: 'system',
              data: '\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n✅ Work Done - Final chunks received\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n',
              is_final: true
            })
          }
          
          return sortedLogs
        })
        
        // Update last chunk index ref (skip system messages with chunk_index -1)
        const maxIndex = Math.max(...newLogs.filter((log: LogChunk) => log.chunk_index >= 0).map((log: LogChunk) => log.chunk_index))
        if (maxIndex > currentIndex) {
          lastChunkIndexRef.current = maxIndex
        }
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error)
    }
  }, [command.command_id, commandStatus])

  // Initial load
  useEffect(() => {
    const loadInitialLogs = async () => {
      try {
        const data = await api.getCommandLogs(command.command_id)
        if (data.logs && Array.isArray(data.logs) && data.logs.length > 0) {
          const sortedLogs = data.logs.sort((a, b) => a.chunk_index - b.chunk_index)
          
          // Check if any logs are final chunks
          const hasFinal = sortedLogs.some(log => log.is_final === true)
          if (hasFinal) {
            setHasFinalChunk(true)
            shouldPollRef.current = false
            
            // Add "Work Done" message to console
            sortedLogs.push({
              chunk_index: -1,
              stream: 'system',
              data: '\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n✅ Work Done - Final chunks received\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n',
              is_final: true
            })
          }
          
          setLogs(sortedLogs)
          const maxIndex = Math.max(...sortedLogs.filter(log => log.chunk_index >= 0).map(log => log.chunk_index))
          lastChunkIndexRef.current = maxIndex
        }
      } catch (error) {
        console.error('Failed to load initial logs:', error)
      }
    }
    loadInitialLogs()
  }, [command.command_id])

  // Polling interval - starts after initial load
  useEffect(() => {
    // Clear any existing interval
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current)
      pollingIntervalRef.current = null
    }

    // Don't start polling if:
    // - We've received final chunks
    // - Command is already finished
    // - Polling is disabled
    if (hasFinalChunk || 
        commandStatus === 'success' || 
        commandStatus === 'failed' || 
        commandStatus === 'timeout' ||
        !shouldPollRef.current) {
      return
    }

    // Start polling immediately, then every 1 second
    fetchLogs()
    pollingIntervalRef.current = setInterval(fetchLogs, 1000)

    return () => {
      if (pollingIntervalRef.current) {
        clearInterval(pollingIntervalRef.current)
        pollingIntervalRef.current = null
      }
    }
  }, [command.command_id, commandStatus, hasFinalChunk, fetchLogs])

  const renderLogLine = (log: LogChunk, index: number) => {
    // Special rendering for system messages (work done)
    if (log.stream === 'system') {
      return (
        <div
          key={`${log.chunk_index}-${log.stream}-${index}`}
          className="font-mono text-sm whitespace-pre-wrap text-green-400 font-bold my-2"
        >
          {log.data}
        </div>
      )
    }
    
    const isStdErr = log.stream === 'stderr'
    return (
      <div
        key={`${log.chunk_index}-${log.stream}-${index}`}
        className={`font-mono text-sm whitespace-pre-wrap ${
          isStdErr ? 'text-red-400' : 'text-green-400'
        }`}
      >
        {log.data}
      </div>
    )
  }

  const getStatusBadge = (status: string) => {
    // Map backend statuses to UI-friendly labels (queued state removed)
    const statusMap: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "success" | "outline" }> = {
      running: { label: 'Live', variant: 'secondary' },
      streaming: { label: 'Live', variant: 'secondary' },
      success: { label: 'Done', variant: 'success' },
      failed: { label: 'Finished', variant: 'destructive' },
      timeout: { label: 'Finished', variant: 'destructive' },
    }
    
    const statusInfo = statusMap[status] || { label: status, variant: 'default' as const }
    return <Badge variant={statusInfo.variant}>{statusInfo.label}</Badge>
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <Card className="w-full max-w-6xl max-h-[90vh] flex flex-col bg-gray-900 border-gray-700">
        <CardHeader className="flex-shrink-0 border-b border-gray-700 bg-gray-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div>
                <CardTitle className="text-white flex items-center gap-2">
                  <span>Command Execution</span>
                  {getStatusBadge(commandStatus)}
                  {!hasFinalChunk && (commandStatus === 'running' || commandStatus === 'streaming') && (
                    <span className="flex items-center gap-1 text-xs text-gray-400">
                      <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                      Live
                    </span>
                  )}
                  {hasFinalChunk && (
                    <span className="flex items-center gap-1 text-xs text-green-400">
                      <div className="w-2 h-2 bg-green-500 rounded-full" />
                      Work Done
                    </span>
                  )}
                </CardTitle>
                <CardDescription className="text-gray-400 mt-1">
                  <code className="text-xs">{currentCommand.command_id}</code>
                  {' • '}
                  <code className="text-xs">{(currentCommand.payload as any)?.cmd || 'N/A'}</code>
                </CardDescription>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {commandStatus !== 'success' && commandStatus !== 'failed' && commandStatus !== 'timeout' && !hasFinalChunk && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    shouldPollRef.current = !shouldPollRef.current
                    if (shouldPollRef.current) {
                      fetchLogs()
                      pollingIntervalRef.current = setInterval(fetchLogs, 1000)
                    } else {
                      if (pollingIntervalRef.current) {
                        clearInterval(pollingIntervalRef.current)
                        pollingIntervalRef.current = null
                      }
                    }
                  }}
                  className="text-gray-400 hover:text-white"
                >
                  {shouldPollRef.current ? (
                    <>
                      <Square className="w-4 h-4 mr-2" />
                      Pause
                    </>
                  ) : (
                    <>
                      <RefreshCw className="w-4 h-4 mr-2" />
                      Resume
                    </>
                  )}
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon"
                onClick={onClose}
                className="text-gray-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="flex-1 overflow-hidden p-0">
          <div
            ref={scrollContainerRef}
            className="h-full overflow-y-auto bg-gray-950 p-4 font-mono text-sm"
            style={{ fontFamily: 'Monaco, Menlo, "Ubuntu Mono", Consolas, "source-code-pro", monospace' }}
          >
            {logs.length === 0 ? (
              <div className="text-gray-500 flex items-center gap-2">
                <div className="w-2 h-2 bg-gray-500 rounded-full animate-pulse" />
                Waiting for output...
              </div>
            ) : (
              logs.map((log, index) => renderLogLine(log, index))
            )}
            <div ref={consoleEndRef} />
          </div>
        </CardContent>
        <div className="flex-shrink-0 border-t border-gray-700 bg-gray-800 px-4 py-2 flex items-center justify-between">
          <div className="text-xs text-gray-400">
            {logs.length} chunk{logs.length !== 1 ? 's' : ''}
            {hasFinalChunk && ' • Final chunks received'}
            {(commandStatus === 'running' || commandStatus === 'streaming') && !hasFinalChunk && ' • Command running...'}
            {commandStatus === 'success' && ` • Exit code: ${currentCommand.exit_code || 0}`}
            {(commandStatus === 'failed' || commandStatus === 'timeout') && ` • Error: ${currentCommand.error_msg || 'Unknown error'}`}
          </div>
          <div className="text-xs text-gray-500">
            {hasFinalChunk ? 'Work completed' : shouldPollRef.current ? 'Polling every 1s' : 'Paused'}
          </div>
        </div>
      </Card>
    </div>
  )
}

