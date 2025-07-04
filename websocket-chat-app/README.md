# Çeting

A real-time chat application built with Go backend and vanilla JavaScript frontend, featuring a Discord-like interface design.

## Features

- 🚀 Real-time messaging with WebSocket connections
- 💬 Multiple chat channels (genel, rastgele, oyun)
- 👤 User authentication with custom usernames
- 🎨 Discord-inspired modern UI design
- 🔄 Automatic reconnection on connection loss
- 🐳 Docker support for easy deployment
- 📱 Responsive design

## Technology Stack

- **Backend**: Go with Gorilla WebSocket
- **Frontend**: HTML5, CSS3, Vanilla JavaScript
- **WebSocket**: Real-time bidirectional communication
- **Containerization**: Docker & Docker Compose

## Project Structure

```
websocket-chat-app/
├── main.go              # Go server with WebSocket handlers
├── index.html           # Frontend application
├── go.mod              # Go module dependencies
├── docker-compose.yml  # Docker compose configuration
├── Dockerfile          # Docker build instructions
└── README.md           # Project documentation
```

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (for containerized deployment)
- Modern web browser with WebSocket support

## Installation & Setup

### Method 1: Local Development

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd websocket-chat-app
   ```

2. **Install Go dependencies**
   ```bash
   go mod tidy
   ```

3. **Run the application**
   ```bash
   go run main.go
   ```

4. **Access the application**
   - Open your browser and navigate to `http://localhost:8080`
   - Enter your username to join the chat

### Method 2: Docker Deployment

1. **Build and run with Docker Compose**
   ```bash
   docker-compose up --build
   ```

2. **Access the application**
   - Open your browser and navigate to `http://localhost:8080`

## Usage

1. **Join the Chat**
   - Enter your desired username in the login modal
   - Click "Sohbete Katıl" to join

2. **Switch Channels**
   - Click on different channels in the sidebar (genel, rastgele, oyun)
   - Each channel maintains separate message history

3. **Send Messages**
   - Type your message in the input field
   - Press Enter or click "Gönder" to send
   - Messages appear in real-time for all connected users

4. **User Interface**
   - Server list on the left (Discord-style)
   - Channel list in the middle sidebar
   - Main chat area with message history
   - User panel at the bottom of sidebar

## API Endpoints

- `GET /` - Serves the main HTML application
- `GET /ws` - WebSocket endpoint for real-time communication

## WebSocket Message Format

Messages are sent as JSON objects:

```json
{
  "username": "john_doe",
  "message": "Hello everyone!",
  "timestamp": "2024-01-15T10:30:00Z",
  "channel": "genel"
}
```

## Features in Detail

### Real-time Communication
- Instant message delivery using WebSocket protocol
- Automatic connection status monitoring
- Reconnection mechanism with exponential backoff

### Multi-channel Support
- Three default channels: genel, rastgele, oyun
- Channel-specific message filtering
- Easy channel switching with persistent state

### User Experience
- Discord-inspired dark theme
- Responsive design for different screen sizes
- Message timestamps and user avatars
- System notifications for connection status

### Error Handling
- Connection failure notifications
- Automatic reconnection attempts
- Graceful degradation on network issues

## Docker Configuration

The application includes Docker support with:
- Multi-stage build for optimized image size
- Port mapping (8080:8080)
- Automatic restart policy
- Volume mounting for development

## Development

### Adding New Features

1. **Backend Changes**: Modify `main.go` for server-side logic
2. **Frontend Changes**: Update `index.html` for UI/UX improvements
3. **Styling**: CSS is embedded in the HTML file for simplicity

### Environment Variables

The application can be configured with environment variables:
- `PORT`: Server port (default: 8080)

## Browser Compatibility

- Chrome 16+
- Firefox 11+
- Safari 7+
- Edge 12+
- Opera 12.1+

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is open source and available under the [MIT License](LICENSE).

## Troubleshooting

### Common Issues

1. **WebSocket Connection Failed**
   - Check if the server is running on port 8080
   - Verify firewall settings
   - Ensure browser supports WebSocket

2. **Messages Not Appearing**
   - Check browser console for JavaScript errors
   - Verify you're in the correct channel
   - Refresh the page to reset connection

3. **Docker Issues**
   - Ensure Docker daemon is running
   - Check port 8080 is not in use by another application
   - Run `docker-compose logs` for debugging

## Support

For support and questions:
- Check the [Issues](issues) section
- Create a new issue for bug reports
- Contribute to discussions and improvements

---

Built with ❤️ using Go and WebSocket