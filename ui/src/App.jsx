import React, { useState, useRef, useEffect } from 'react';
import { Send, Loader2, AlertCircle, RefreshCw, Settings, User, Bot, CheckCircle, Zap } from 'lucide-react';

// Mock implementation of useMcp hook for demonstration
const useMcp = ({ url, clientName, autoReconnect }) => {
  const [state, setState] = useState('connecting');
  const [tools, setTools] = useState([]);
  const [error, setError] = useState(null);
  
  useEffect(() => {
    const timer = setTimeout(() => {
      setState('ready');
      setTools([
        { name: 'search', description: 'Search for information across the web' },
        { name: 'calculate', description: 'Perform mathematical calculations' },
        { name: 'weather', description: 'Get current weather information' },
        { name: 'translate', description: 'Translate text between languages' }
      ]);
    }, 2000);
    
    return () => clearTimeout(timer);
  }, []);
  
  const callTool = async (name, args) => {
    await new Promise(resolve => setTimeout(resolve, 1000));
    return {
      success: true,
      result: `Tool "${name}" executed successfully`,
      data: `Mock response from ${name} tool with args: ${JSON.stringify(args)}`
    };
  };
  
  const retry = () => {
    setState('connecting');
    setError(null);
    setTimeout(() => setState('ready'), 2000);
  };
  
  const authenticate = () => console.log('Authenticate called');
  const clearStorage = () => console.log('Clear storage called');
  
  return { state, tools, error, callTool, retry, authenticate, clearStorage };
};

const App = () => {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [serverUrl, setServerUrl] = useState('https://your-mcp-server.com');
  const [clientName, setClientName] = useState('MCP Chat Client');
  
  const messagesEndRef = useRef(null);
  
  const {
    state,
    tools,
    error,
    callTool,
    retry,
    authenticate,
    clearStorage
  } = useMcp({
    url: serverUrl,
    clientName: clientName,
    autoReconnect: true
  });

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const addMessage = (content, sender = 'user', toolCall = null) => {
    const message = {
      id: Date.now(),
      content,
      sender,
      timestamp: new Date(),
      toolCall
    };
    setMessages(prev => [...prev, message]);
    return message;
  };

  const handleSendMessage = async () => {
    if (!input.trim() || state !== 'ready') return;
    
    const userMessage = input.trim();
    setInput('');
    addMessage(userMessage, 'user');
    setIsLoading(true);
    
    try {
      if (userMessage.startsWith('/')) {
        const [command, ...args] = userMessage.slice(1).split(' ');
        const toolName = command.toLowerCase();
        
        const tool = tools.find(t => t.name === toolName);
        if (tool) {
          const toolArgs = args.length > 0 ? { query: args.join(' ') } : {};
          
          addMessage(`Calling tool: ${toolName}...`, 'assistant');
          const result = await callTool(toolName, toolArgs);
          
          addMessage(
            `Tool result: ${result.result}\n\nData: ${result.data}`, 
            'assistant', 
            { name: toolName, args: toolArgs, result }
          );
        } else {
          addMessage(
            `Unknown tool: ${toolName}. Available tools: ${tools.map(t => t.name).join(', ')}`,
            'assistant'
          );
        }
      } else {
        addMessage(
          `Echo: ${userMessage}\n\nYou can use tools by typing /${`toolName`} [args]. Available tools: ${tools.map(t => t.name).join(', ')}`,
          'assistant'
        );
      }
    } catch (err) {
      addMessage(`Error: ${err.message}`, 'assistant');
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  const formatTimestamp = (date) => {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className="chat-container" style={{ fontFamily: 'inherit' }}>
      
      {/* Header */}
      <div className="header">
        <div>
          <h1>MCP Chat Interface</h1>
          <p>Model Context Protocol Client</p>
        </div>
        <button
          className="btn"
          onClick={() => setShowSettings(!showSettings)}
        >
          <Settings size={20} />
        </button>
      </div>

      {/* Settings Panel */}
      {showSettings && (
        <div className="settings-panel">
          <div className="form-grid">
            <div className="form-group">
              <label>Server URL</label>
              <input
                type="text"
                value={serverUrl}
                onChange={(e) => setServerUrl(e.target.value)}
                className="form-control"
              />
            </div>
            <div className="form-group">
              <label>Client Name</label>
              <input
                type="text"
                value={clientName}
                onChange={(e) => setClientName(e.target.value)}
                className="form-control"
              />
            </div>
          </div>
          <button onClick={clearStorage} className="btn btn-secondary">
            Clear Storage
          </button>
        </div>
      )}

      {/* Connection Status */}
      <div className="status-panel">
        {state === 'connecting' && (
          <div className="status-connecting">
            <Loader2 size={16} className="spin" />
            <span>Connecting to MCP server...</span>
          </div>
        )}
        
        {state === 'failed' && (
          <div className="status-failed">
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <AlertCircle size={16} />
              <span>Connection failed: {error}</span>
            </div>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button onClick={authenticate} className="btn btn-secondary">
                Authenticate
              </button>
              <button onClick={retry} className="btn btn-primary">
                Retry
              </button>
            </div>
          </div>
        )}
        
        {state === 'ready' && (
          <div className="status-ready">
            <div className="pulse"></div>
            <span>Connected • {tools.length} tools available</span>
          </div>
        )}
      </div>

      {/* Available Tools */}
      {state === 'ready' && tools.length > 0 && (
        <div className="tools-panel">
          <p style={{ fontSize: '0.875rem', fontWeight: '500', margin: '0 0 0.5rem 0' }}>
            Available Tools:
          </p>
          <div className="tools-list">
            {tools.map(tool => (
              <span key={tool.name} className="tool-tag" title={tool.description}>
                /{tool.name}
              </span>
            ))}
          </div>
          <p style={{ fontSize: '0.75rem', color: 'var(--gray-600)', margin: '0.5rem 0 0 0' }}>
            Use tools by typing /{`toolName`} followed by arguments
          </p>
        </div>
      )}

      {/* Messages */}
      <div className="messages">
        {messages.length === 0 && state === 'ready' && (
          <div className="empty-state">
            <p>Welcome to MCP Chat Interface!</p>
            <p style={{ fontSize: '0.875rem', marginTop: '0.5rem' }}>
              Start chatting or use tools with /{`toolName`} commands
            </p>
          </div>
        )}
        
        {messages.map(message => (
          <div key={message.id} className={`message ${message.sender}`}>
            <div className={`message-bubble ${message.sender}`}>
              <div className="message-header">
                {message.sender === 'user' ? <User size={16} /> : <Bot size={16} />}
                <span>{formatTimestamp(message.timestamp)}</span>
              </div>
              <div className="message-content">{message.content}</div>
              {message.toolCall && (
                <div className="tool-call">
                  <strong>Tool Call:</strong> {message.toolCall.name}
                </div>
              )}
            </div>
          </div>
        ))}
        
        {isLoading && (
          <div className="message assistant">
            <div className="message-bubble assistant">
              <div className="message-header">
                <Bot size={16} />
                <Loader2 size={16} className="spin" />
                <span>Processing...</span>
              </div>
            </div>
          </div>
        )}
        
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="input-area">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={handleKeyPress}
          placeholder={
            state === 'ready'
              ? "Type a message or use /toolName command..."
              : "Waiting for connection..."
          }
          disabled={state !== 'ready' || isLoading}
          className="message-input"
          rows="1"
        />
        <button
          onClick={handleSendMessage}
          disabled={!input.trim() || state !== 'ready' || isLoading}
          className="send-btn"
        >
          <Send size={16} />
        </button>
      </div>
    </div>
  );
};

export default App;