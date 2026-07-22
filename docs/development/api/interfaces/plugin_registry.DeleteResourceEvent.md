[API](../API.md) / [plugin/registry](../modules/plugin_registry.md) / DeleteResourceEvent

# Interface: DeleteResourceEvent

[plugin/registry](../modules/plugin_registry.md).DeleteResourceEvent

Event fired when a resource is to be deleted.

## Hierarchy

- [`K8senseEvent`](plugin_registry.K8senseEvent.md)<`K8senseEventType.DELETE_RESOURCE`\>

  ↳ **`DeleteResourceEvent`**

## Properties

### data

• **data**: `Object`

#### Type declaration

| Name | Type | Description |
| :------ | :------ | :------ |
| `resource` | `any` | The resource for which the deletion was called. |
| `status` | `CONFIRMED` | What exactly this event represents. 'CONFIRMED' when the user confirms the deletion of a resource. For now only 'CONFIRMED' is sent. |

#### Overrides

[K8senseEvent](plugin_registry.K8senseEvent.md).[data](plugin_registry.K8senseEvent.md#data)

#### Defined in

[redux/k8senseEventSlice.ts:85](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L85)

___

### type

• **type**: `DELETE_RESOURCE`

#### Inherited from

[K8senseEvent](plugin_registry.K8senseEvent.md).[type](plugin_registry.K8senseEvent.md#type)

#### Defined in

[redux/k8senseEventSlice.ts:68](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L68)
