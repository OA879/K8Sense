[API](../API.md) / [plugin/registry](../modules/plugin_registry.md) / TerminalEvent

# Interface: TerminalEvent

[plugin/registry](../modules/plugin_registry.md).TerminalEvent

Event fired when using the terminal.

## Properties

### data

• **data**: `Object`

#### Type declaration

| Name | Type | Description |
| :------ | :------ | :------ |
| `resource?` | `any` | The resource for which the terminal was opened (currently this only happens for Pod instances). |
| `status` | `OPENED` \| `CLOSED` | What exactly this event represents. 'OPEN' when the terminal is opened. 'CLOSED' when it is closed. |

#### Defined in

[redux/k8senseEventSlice.ts:163](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L163)

___

### type

• **type**: `TERMINAL`

#### Defined in

[redux/k8senseEventSlice.ts:162](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L162)
