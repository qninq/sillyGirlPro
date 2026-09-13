import message from "ant-design-vue/es/message";
import { Modal } from "ant-design-vue";
import { createVNode } from "vue";
import { TriangleAlert } from "lucide-vue-next";

// 危险操作（删除 / 恢复等）的居中确认对话框：带警示图标、说明与红色确认按钮。
// onOk 抛错时提示错误并保持弹窗打开，等待重试。
export function confirmDanger(options: {
  title: string;
  description?: string;
  okText?: string;
  onOk: () => Promise<void> | void;
}) {
  Modal.confirm({
    title: options.title,
    icon: createVNode(TriangleAlert, { size: 22, color: "#faad14" }),
    content: options.description,
    okText: options.okText || "删除",
    okType: "danger",
    cancelText: "取消",
    centered: true,
    width: 420,
    onOk: async () => {
      try {
        await options.onOk();
      } catch (error) {
        message.error(error instanceof Error ? error.message : "操作失败");
        throw error;
      }
    },
  });
}
