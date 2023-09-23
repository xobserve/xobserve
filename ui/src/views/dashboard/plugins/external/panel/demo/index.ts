/* 
This panel is for demonstration purpose, it is an external plugin, auto generated by Datav.

The origin plugin files is in https://github.com/data-observe/datav/tree/main/ui/external-plugins
*/

import { PanelPluginComponents } from "types/plugins/plugin";
import DemoPanel from "./Demo";
import DemoPanelEditor from "./Editor";
import OverrideEditor, { OverrideRules, getOverrideTargets } from "./OverrideEditor";

const demoComponents: PanelPluginComponents = {
    panel: DemoPanel,
    editor: DemoPanelEditor,
    overrideEditor: OverrideEditor,
    overrideRules: OverrideRules,
    getOverrideTargets: getOverrideTargets
}

export default  demoComponents