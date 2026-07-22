[API](../API.md) / [plugin/registry](../modules/plugin_registry.md) / EditResourceEvent

# Interface: EditResourceEvent

[plugin/registry](../modules/plugin_registry.md).EditResourceEvent

Event fired when editing a resource.

## Properties

### data

• **data**: `Object`

#### Type declaration

| Name | Type | Description |
| :------ | :------ | :------ |
| `resource` | `any` | The resource for which the deletion was called. |
| `status` | `OPENED` \| `CLOSED` | What exactly this event represents. 'OPEN' when the edit dialog is opened. 'CLOSED' when it is closed. |

#### Defined in

[redux/k8senseEventSlice.ts:100](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L100)

___

### type

• **type**: `EDIT_RESOURCE`

#### Defined in

[redux/k8senseEventSlice.ts:99](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L99)
