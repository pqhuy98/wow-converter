import { Plus, Sword, Trash2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent,
  CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  allAttachments,
  AttachItem, Character,
} from '@/lib/models/export-character.model';

import { TooltipHelp } from '../common/tooltip-help';
import { AttachmentSelector } from './attachment-selector';
import { RefInput } from './ref-input';

const tooltips = {
  itemReference: 'The item to attach - can be a Wowhead URL, local file inside wow.export folder, or Display ID.',
  itemRemove: 'Remove the item',
  attachmentPoint: 'Where on the character model this item will be attached',
  itemScale: 'Additional scale multiplier for this specific item (1.0 = no change). Firstly the item will be scaled to match the character, then this multiplier will be applied.',
};

export function AttachItems({
  character,
  setCharacter,
  removeAttachItem,
  updateAttachItem,
  addAttachItem,
}: {
  character: Character
  setCharacter: React.Dispatch<React.SetStateAction<Character>>
  removeAttachItem: (id: number) => void
  updateAttachItem: (id: number, item: AttachItem) => void
  addAttachItem: () => void
}) {
  return (
    <Card>
      <CardHeader className="pb-4">
        <CardTitle className="flex items-center gap-2 text-lg">
          <Sword className="h-5 w-5" />
          Attached Items
        </CardTitle>
        <CardDescription>Add weapons and other items to attach to the character</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="space-y-3">
          {Object.entries(character.attachItems || {}).map(([id, item]) => {
            const attachmentId = Number(id);
            const usedIds = new Set(Object.keys(character.attachItems || {}).map(Number));
            const attachmentName = allAttachments.find((att) => att.id === attachmentId)?.name
              || 'Unknown';

            return (
              <Card key={id} className="p-3 bg-card border-border">
                <div className="flex items-start justify-between mb-3">
                  <Badge variant="secondary" className="text-xs">
                    {attachmentName} ({attachmentId})
                  </Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeAttachItem(attachmentId)}
                    className="text-red-500 hover:text-red-700 h-6 w-6 p-0"
                  >
                    <TooltipHelp trigger={<Trash2 className="h-3 w-3" />} tooltips={tooltips.itemRemove}/>
                  </Button>
                </div>

                <div className="space-y-3">
                  <RefInput
                    value={item.path}
                    onChange={(path) => {
                      updateAttachItem(attachmentId, { ...item, path });
                    }}
                    label="Item Reference"
                    tooltip={tooltips.itemReference}
                    category="item"
                  />

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <Label className="text-sm">Attachment Point</Label>
                        <TooltipHelp tooltips={tooltips.attachmentPoint} />
                      </div>
                      <AttachmentSelector
                        value={attachmentId}
                        onChange={(newId) => {
                          // Move the item to the new attachment ID
                          setCharacter((prev) => {
                            const newAttachItems = { ...prev.attachItems };
                            delete newAttachItems[attachmentId];
                            newAttachItems[newId] = item;
                            return { ...prev, attachItems: newAttachItems };
                          });
                        }}
                        usedIds={usedIds}
                      />
                    </div>

                    <div className="flex items-end gap-3">
                      <div className="flex-1 space-y-2">
                        <div className="flex items-center gap-2">
                          <Label htmlFor={`scale-${id}`} className="text-sm">
                            Scale
                          </Label>
                          <TooltipHelp tooltips={tooltips.itemScale} />
                        </div>
                        <Input
                          id={`scale-${id}`}
                          type="number"
                          step="0.1"
                          placeholder="1.0"
                          value={item.scale || ''}
                          onChange={(e) => updateAttachItem(attachmentId, {
                            ...item,
                            scale: Number.parseFloat(e.target.value) || undefined,
                          })
                          }
                          className=""
                        />
                      </div>
                    </div>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>

        <Button onClick={addAttachItem} variant="outline" className="w-full bg-transparent">
          <Plus className="h-4 w-4 mr-2" />
          Add Attached Item
        </Button>
      </CardContent>
    </Card>
  );
}
