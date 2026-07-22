[API](../API.md) / [plugin/lib](../modules/plugin_lib.md) / Plugin

# Class: Plugin

[plugin/lib](../modules/plugin_lib.md).Plugin

Plugins may call K8sense.registerPlugin(pluginId: string, pluginObj: Plugin) to register themselves.

They will have their initialize(register) method called at plugin initialization time.

## Constructors

### constructor

• **new Plugin**()

## Methods

### initialize

▸ `Abstract` **initialize**(`register`): `boolean` \| `void`

initialize is called for each plugin with a Registry which gives the plugin methods for doing things.

**`see`** Registry

#### Parameters

| Name | Type |
| :------ | :------ |
| `register` | [`Registry`](plugin_registry.Registry.md) |

#### Returns

`boolean` \| `void`

The return code is not used, but used to be required.

#### Defined in

[plugin/lib.ts:49](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/plugin/lib.ts#L49)
