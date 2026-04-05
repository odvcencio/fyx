import * as path from 'path';
import { workspace, ExtensionContext } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: ExtensionContext) {
    const config = workspace.getConfiguration('fyx');
    const serverCommand = config.get<string>('serverPath', 'fyxc');

    const serverOptions: ServerOptions = {
        run: { command: serverCommand, args: ['lsp'], transport: TransportKind.stdio },
        debug: { command: serverCommand, args: ['lsp'], transport: TransportKind.stdio },
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'fyx' }],
        synchronize: {
            fileEvents: workspace.createFileSystemWatcher('**/*.fyx'),
        },
    };

    client = new LanguageClient(
        'fyxLanguageServer',
        'Fyx Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
