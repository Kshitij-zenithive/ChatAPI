# ChatAPI

ChatAPI is a simple chat application built with Go. It supports WebSocket communication, user mentions, and message broadcasting. This project demonstrates the implementation of a real-time chat server with features like message history, user mentions, and automatic responses.

## Features

- **Real-time Communication**: Uses WebSockets to provide real-time chat functionality.
- **User Mentions**: Supports mentioning users in messages using `@username`.
- **Message History**: New users receive the chat history upon connecting.
- **Automatic Responses**: Auto-responds to mentions for demonstration purposes.
- **Database Integration**: Stores messages and user information in a database.
- **Simulated Users**: Includes a simulation of automated chat between virtual users for testing.

## Installation

To get started with ChatAPI, follow these steps:

1. **Clone the repository**:
    ```bash
    git clone https://github.com/Kshitij-zenithive/ChatAPI.git
    cd ChatAPI
    ```

2. **Install dependencies**:
    Make sure you have Go installed on your system. Then, run:
    ```bash
    go mod tidy
    ```

3. **Configure the database**:
    Ensure that you have a PostgreSQL database set up and update the database configuration in the `database` package.

4. **Run the server**:
    ```bash
    go run main.go
    ```

## Usage

Once the server is running, you can test the chat application by opening the chat test interface in your browser:

1. **Open the chat test interface**:
    ```
    http://localhost:5000/chat-test
    ```

2. **Connect to the chat**:
    - Enter a username and click "Connect" to join the chat.
    - You can mention other users by typing `@` followed by their username.

## Project Structure

- **main.go**: The main entry point of the application, containing the server setup and WebSocket handling.
- **go.mod**: The Go module file, listing the dependencies used in the project.
- **database**: Contains the database initialization and operations.

## Key Components

- **ChatHub**: Manages the set of active clients and broadcasts messages.
- **ChatClient**: Represents a single WebSocket connection.
- **Message Handling**: Processes incoming messages and handles user mentions.
- **HTML Templates**: Provides a simple HTML interface for testing the chat functionality.

## Recent Commits

- **Complete implementation of CHAT with @mentions**: [Commit 50591d9](https://github.com/Kshitij-zenithive/ChatAPI/commit/50591d9fa9b3ea44c1b8fa955ca6402547d04e86)
- **Initial Commit**: [Commit e9e6faaf](https://github.com/Kshitij-zenithive/ChatAPI/commit/e9e6faafddfe06e133a59bbac1a3f0c4cc0affb5)

## Contributors

- **Kshitij-zenithive**: [GitHub Profile](https://github.com/Kshitij-zenithive)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
