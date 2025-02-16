package ipc

import (
	"fmt"

	"github.com/wailsapp/wails/lib/interfaces"
	"github.com/wailsapp/wails/lib/logger"
	"github.com/wailsapp/wails/lib/messages"
)

// Manager manages the IPC subsystem
type Manager struct {
	renderer     interfaces.Renderer // The renderer
	messageQueue chan *ipcMessage
	// quitChannel  chan struct{}
	// signals      chan os.Signal
	log            *logger.CustomLogger
	eventManager   interfaces.EventManager
	bindingManager interfaces.BindingManager
}

// NewManager creates a new IPC Manager
func NewManager() interfaces.IPCManager {
	result := &Manager{
		messageQueue: make(chan *ipcMessage, 100),
		// 		quitChannel:  make(chan struct{}),
		// 		signals:      make(chan os.Signal, 1),
		log: logger.NewCustomLogger("IPC"),
	}
	return result
}

// BindRenderer sets the renderer, returns the dispatch function
func (i *Manager) BindRenderer(renderer interfaces.Renderer) {
	i.renderer = renderer
}

// Start the IPC Manager
func (i *Manager) Start(eventManager interfaces.EventManager, bindingManager interfaces.BindingManager) {

	// Store manager references
	i.eventManager = eventManager
	i.bindingManager = bindingManager

	i.log.Info("Starting")
	// signal.Notify(manager.signals, os.Interrupt)
	go func() {
		running := true
		for running {
			select {
			case incomingMessage := <-i.messageQueue:
				i.log.DebugFields("Processing message", logger.Fields{
					"1D": &incomingMessage,
				})
				switch incomingMessage.Type {
				case "call":
					callData := incomingMessage.Payload.(*messages.CallData)
					i.log.DebugFields("Processing call", logger.Fields{
						"1D":          &incomingMessage,
						"bindingName": callData.BindingName,
						"data":        callData.Data,
					})
					go func() {
						result, err := bindingManager.ProcessCall(callData)
						i.log.DebugFields("processed call", logger.Fields{"result": result, "err": err})
						if err != nil {
							incomingMessage.ReturnError(err.Error())
						} else {
							incomingMessage.ReturnSuccess(result)
						}
						i.log.DebugFields("Finished processing call", logger.Fields{
							"1D": &incomingMessage,
						})
					}()
				case "event":

					// Extract event data
					eventData := incomingMessage.Payload.(*messages.EventData)

					// Log
					i.log.DebugFields("Processing event", logger.Fields{
						"name": eventData.Name,
						"data": eventData.Data,
					})

					// Push the event to the event manager
					i.eventManager.PushEvent(eventData)

					// Log
					i.log.DebugFields("Finished processing event", logger.Fields{
						"name": eventData.Name,
					})
				case "log":
					logdata := incomingMessage.Payload.(*messages.LogData)
					switch logdata.Level {
					case "info":
						logger.GlobalLogger.Info(logdata.Message)
					case "debug":
						logger.GlobalLogger.Debug(logdata.Message)
					case "warning":
						logger.GlobalLogger.Warn(logdata.Message)
					case "error":
						logger.GlobalLogger.Error(logdata.Message)
					case "fatal":
						logger.GlobalLogger.Fatal(logdata.Message)
					default:
						logger.ErrorFields("Invalid log level sent", logger.Fields{
							"level":   logdata.Level,
							"message": logdata.Message,
						})
					}
				default:
					i.log.Debugf("bad message sent to MessageQueue! Unknown type: %s", incomingMessage.Type)
				}

				// Log
				i.log.DebugFields("Finished processing message", logger.Fields{
					"1D": &incomingMessage,
				})
				// 			case <-manager.quitChannel:
				// 				Debug("[MessageQueue] Quit caught")
				// 				running = false
				// 			case <-manager.signals:
				// 				Debug("[MessageQueue] Signal caught")
				// 				running = false
			}
		}
		i.log.Debug("Stopping")
	}()
}

// Dispatch receives JSON encoded messages from the renderer.
// It processes the message to ensure that it is valid and places
// the processed message on the message queue
func (i *Manager) Dispatch(message string) {

	// Create a new IPC Message
	incomingMessage, err := newIPCMessage(message, i.SendResponse)
	if err != nil {
		i.log.ErrorFields("Could not understand incoming message! ", map[string]interface{}{
			"message": message,
			"error":   err,
		})
		return
	}

	// Put message on queue
	i.log.DebugFields("Message received", map[string]interface{}{
		"type":    incomingMessage.Type,
		"payload": incomingMessage.Payload,
	})

	// Put incoming message on the message queue
	i.messageQueue <- incomingMessage
}

// SendResponse sends the given response back to the frontend
func (i *Manager) SendResponse(response *ipcResponse) error {

	// Serialise the Message
	data, err := response.Serialise()
	if err != nil {
		fmt.Printf(err.Error())
		return err
	}

	// Call back to the front end
	return i.renderer.Callback(data)
}
